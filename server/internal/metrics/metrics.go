// Package metrics 提供最小的进程内计数器，用于定位卡住或断线的对局。
//
// 它只做累加，不依赖外部监控系统，也不参与任何规则或状态计算。
// /metrics 以 "name value" 每行一条输出，便于人工查看和日志采集。
package metrics

import (
	"fmt"
	"io"
	"sync/atomic"
)

// Registry 是一组进程级计数器。所有方法并发安全。
type Registry struct {
	connectionsOpened atomic.Int64
	connectionsActive atomic.Int64
	matchesStarted    atomic.Int64
	commandsAccepted  atomic.Int64
	commandsRejected  atomic.Int64
	eventsEmitted     atomic.Int64
	broadcastsSent    atomic.Int64
}

// Sample 是一条指标快照。
type Sample struct {
	Name  string
	Value int64
}

func New() *Registry {
	return &Registry{}
}

// ConnectionOpened 记录一条新连接（同时增加活跃连接数）。
func (r *Registry) ConnectionOpened() {
	r.connectionsOpened.Add(1)
	r.connectionsActive.Add(1)
}

// ConnectionClosed 记录一条连接关闭。
func (r *Registry) ConnectionClosed() {
	r.connectionsActive.Add(-1)
}

// MatchStarted 记录一局开始。
func (r *Registry) MatchStarted() {
	r.matchesStarted.Add(1)
}

// CommandAccepted 记录一条被接受的指令。
func (r *Registry) CommandAccepted() {
	r.commandsAccepted.Add(1)
}

// CommandRejected 记录一条被拒绝的指令。
func (r *Registry) CommandRejected() {
	r.commandsRejected.Add(1)
}

// EventsEmitted 累加本次结算产生的事件数量。
func (r *Registry) EventsEmitted(count int) {
	if count > 0 {
		r.eventsEmitted.Add(int64(count))
	}
}

// BroadcastSent 记录一次按座位发出的广播。
func (r *Registry) BroadcastSent() {
	r.broadcastsSent.Add(1)
}

// Snapshot 按固定顺序返回指标快照，便于断言与人工查看。
func (r *Registry) Snapshot() []Sample {
	return []Sample{
		{"riftcards_connections_total", r.connectionsOpened.Load()},
		{"riftcards_connections_active", r.connectionsActive.Load()},
		{"riftcards_matches_total", r.matchesStarted.Load()},
		{"riftcards_commands_accepted_total", r.commandsAccepted.Load()},
		{"riftcards_commands_rejected_total", r.commandsRejected.Load()},
		{"riftcards_events_emitted_total", r.eventsEmitted.Load()},
		{"riftcards_broadcasts_total", r.broadcastsSent.Load()},
	}
}

// WriteText 以文本行输出快照。
// 不命名为 WriteTo，避免与 io.WriterTo 的约定签名冲突。
func (r *Registry) WriteText(writer io.Writer) error {
	for _, sample := range r.Snapshot() {
		if _, err := fmt.Fprintf(writer, "%s %d\n", sample.Name, sample.Value); err != nil {
			return err
		}
	}
	return nil
}
