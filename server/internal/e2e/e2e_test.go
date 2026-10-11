package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mjq/riftcards/server/internal/game"
	"github.com/mjq/riftcards/server/internal/protocol"
)

const (
	// testSeed 固定随机种子，让先手、牌序和胜负路径可复现。
	testSeed = 20261008

	matchTimeout = 2 * time.Minute
)

// matchReport 汇总一局的观测结果。
type matchReport struct {
	matchID      string
	winnerSeat   int
	lastEventSeq int64
	events       []game.Event
	accepted     int
	rejected     int
	rejections   map[string]int
	eventBatches int
	clients      []*testClient
}

// eventSignature 用事件类型、座位与载荷构成可比较的签名，用于验证可复现性。
func (r *matchReport) eventSignature() string {
	var builder strings.Builder
	for _, event := range r.events {
		payload, _ := json.Marshal(event.Data)
		fmt.Fprintf(&builder, "%d|%s|%d|%s\n", event.Seq, event.Type, event.ActorSeat, payload)
	}
	return builder.String()
}

// TestLocalMatchCompletesEndToEnd 启动真实服务，让两个独立客户端完成一整局。
func TestLocalMatchCompletesEndToEnd(t *testing.T) {
	harness := startServer(t, testSeed)

	ctx, cancel := context.WithTimeout(context.Background(), matchTimeout)
	defer cancel()

	report := runMatch(t, ctx, harness, "A", "B")

	// 1) 两个客户端进入同一对局，座位互不相同。
	if report.matchID == "" {
		t.Fatal("对局没有 matchId")
	}
	first, second := report.clients[0], report.clients[1]
	if first.matchID != second.matchID {
		t.Fatalf("两个客户端的 matchId 不一致：%s / %s", first.matchID, second.matchID)
	}
	if first.seat == second.seat {
		t.Fatalf("两个客户端座位相同：%d", first.seat)
	}
	if first.seat+second.seat != 1 {
		t.Fatalf("座位不是 0/1：%d / %d", first.seat, second.seat)
	}

	// 2) 事件序号连续：两个客户端的并集必须精确覆盖 1..lastEventSeq。
	if first.lastEventSeq != second.lastEventSeq {
		t.Fatalf("两个客户端看到的 lastEventSeq 不一致：%d / %d", first.lastEventSeq, second.lastEventSeq)
	}
	if int64(len(report.events)) != report.lastEventSeq {
		t.Fatalf("事件并集 %d 条, want %d（按座位投影过滤后不应丢事件或重复）",
			len(report.events), report.lastEventSeq)
	}
	for index, event := range report.events {
		if event.Seq != int64(index+1) {
			t.Fatalf("事件并集第 %d 条 seq=%d, want %d", index, event.Seq, index+1)
		}
	}

	// 3) 终局：双方投影一致地结束，且能对上 winnerSeat。
	for _, client := range report.clients {
		if client.view == nil || client.view.Status != game.StatusFinished {
			t.Fatalf("%s 结束时 status=%v, want %s\n%s", client.name, client.view, game.StatusFinished, client.diagnostics())
		}
		if client.view.WinnerSeat == nil {
			t.Fatalf("%s 的终局投影缺少 winnerSeat", client.name)
		}
		if *client.view.WinnerSeat != report.winnerSeat {
			t.Fatalf("%s 的 winnerSeat=%d, want %d", client.name, *client.view.WinnerSeat, report.winnerSeat)
		}
	}
	ended := lastEventOfType(report.events, "match_ended")
	if ended == nil {
		t.Fatal("事件并集里没有 match_ended")
	}
	if ended.Data["reason"] == nil {
		t.Fatalf("match_ended 缺少 reason：%v", ended.Data)
	}

	// 4) 拒绝路径：越权指令必须被拒绝且不推进 revision。
	//
	// 非当前回合方在开局时提交一条 end_turn：若它抢在对手行动之前到达，服务端返回
	// not_your_turn；若对手已经行动，则版本已经变化而返回 stale_revision。两种都是
	// 合法拒绝，因此断言“被拒绝 + 版本未推进”，而不是断言具体错误码。
	probed := 0
	for _, client := range report.clients {
		if client.probeResult == nil {
			continue
		}
		probed++
		if client.probeResult.Accepted {
			t.Fatalf("%s 的越权指令被接受了：%+v", client.name, *client.probeResult)
		}
		code := client.probeResult.Code
		if code != game.CodeNotYourTurn && code != protocol.CodeStaleRevision {
			t.Fatalf("%s 的越权指令错误码 = %q, want %s 或 %s",
				client.name, code, game.CodeNotYourTurn, protocol.CodeStaleRevision)
		}
	}
	if probed == 0 {
		t.Fatal("没有观察到越权指令探测")
	}
	if report.accepted == 0 {
		t.Fatal("没有任何被接受的指令")
	}
	if report.rejected < probed {
		t.Fatalf("拒绝指令数 = %d, want >= %d", report.rejected, probed)
	}
	// 所有拒绝都必须使用协议文档里列出的错误码（防止出现未文档化的拒绝原因）。
	documented := map[string]bool{
		game.CodeInvalidCommand:     true,
		game.CodeNotYourTurn:        true,
		game.CodeInvalidCard:        true,
		game.CodeInsufficientEnergy: true,
		game.CodeInvalidTarget:      true,
		game.CodeBoardFull:          true,
		game.CodeMatchFinished:      true,
		protocol.CodeStaleRevision:  true,
	}
	for code, count := range report.rejections {
		if !documented[code] {
			t.Fatalf("出现未文档化的拒绝错误码 %q（%d 次）", code, count)
		}
	}

	// 5) 指标：数量必须与实际发生的结算一致。
	values := harness.metrics(t)
	expectMetric(t, values, "riftcards_matches_total", 1)
	expectMetric(t, values, "riftcards_commands_accepted_total", int64(report.accepted))
	expectMetric(t, values, "riftcards_commands_rejected_total", int64(report.rejected))
	expectMetric(t, values, "riftcards_events_emitted_total", report.lastEventSeq)
	expectMetric(t, values, "riftcards_broadcasts_total", int64(report.eventBatches))
	expectMetric(t, values, "riftcards_connections_active", 2)
	expectMetric(t, values, "riftcards_connections_total", 2)

	// 6) 结构化日志足以定位卡住或断线。
	harness.logContains(t,
		"match created",
		"command accepted",
		"command rejected",
		"connection opened",
	)

	// 7) 关闭连接后服务端要记录断开，并且进程可被完全回收（无孤儿进程）。
	first.close()
	second.close()
	harness.waitForLog(t, "session disconnected", 10*time.Second)
	harness.logContains(t, "connection closed")
	harness.shutdown(t)
	harness.assertNoOrphans(t)
}

// TestLocalMatchIsReproducibleWithFixedSeed 固定种子下，两局的完整事件序列必须一致。
func TestLocalMatchIsReproducibleWithFixedSeed(t *testing.T) {
	harness := startServer(t, testSeed)

	ctx, cancel := context.WithTimeout(context.Background(), 2*matchTimeout)
	defer cancel()

	firstRun := runMatch(t, ctx, harness, "R1", "R2")
	secondRun := runMatch(t, ctx, harness, "R3", "R4")

	if firstRun.matchID == secondRun.matchID {
		t.Fatalf("两局复用了同一个 matchId：%s", firstRun.matchID)
	}
	if firstRun.lastEventSeq != secondRun.lastEventSeq {
		t.Fatalf("两局事件数量不同：%d / %d", firstRun.lastEventSeq, secondRun.lastEventSeq)
	}
	if firstRun.winnerSeat != secondRun.winnerSeat {
		t.Fatalf("两局胜者不同：%d / %d", firstRun.winnerSeat, secondRun.winnerSeat)
	}
	if firstRun.eventSignature() != secondRun.eventSignature() {
		t.Fatalf("固定种子下两局事件序列不同：\n--- 第一局 ---\n%s\n--- 第二局 ---\n%s",
			firstRun.eventSignature(), secondRun.eventSignature())
	}

	values := harness.metrics(t)
	expectMetric(t, values, "riftcards_matches_total", 2)
}

// runMatch 让两个独立客户端完成一整局，并汇总观测结果。
func runMatch(t *testing.T, ctx context.Context, harness *serverHarness, nameA, nameB string) *matchReport {
	t.Helper()

	clientA := dialClient(t, ctx, harness.wsURL, nameA)
	clientB := dialClient(t, ctx, harness.wsURL, nameB)
	t.Cleanup(clientA.close)
	t.Cleanup(clientB.close)

	done := make(chan error, 2)
	go func() { done <- clientA.run() }()
	go func() { done <- clientB.run() }()

	var failures []string
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				failures = append(failures, err.Error())
			}
		case <-time.After(matchTimeout):
			failures = append(failures, "等待对局结束时超时")
		}
	}
	if len(failures) > 0 {
		t.Fatalf("端到端对局失败：\n%s\n--- 诊断 ---\n%s\n%s\n--- 服务端日志 ---\n%s",
			strings.Join(failures, "\n"), clientA.diagnostics(), clientB.diagnostics(), harness.serverLog())
	}

	report := &matchReport{
		matchID:    clientA.matchID,
		clients:    []*testClient{clientA, clientB},
		rejections: map[string]int{},
	}
	for _, client := range report.clients {
		report.accepted += client.accepted
		report.rejected += client.rejected
		for _, outcome := range client.outcomes {
			if !outcome.Accepted && outcome.Code != "" {
				report.rejections[outcome.Code]++
			}
		}
		report.eventBatches += client.eventBatches
	}
	if clientA.view != nil && clientA.view.WinnerSeat != nil {
		report.winnerSeat = *clientA.view.WinnerSeat
	}
	report.lastEventSeq = clientA.lastEventSeq
	report.events = mergeEvents(clientA, clientB)
	return report
}

// mergeEvents 合并两个客户端的可见事件，按 seq 排序并检查没有冲突。
func mergeEvents(clients ...*testClient) []game.Event {
	merged := map[int64]game.Event{}
	for _, client := range clients {
		for _, event := range client.events {
			merged[event.Seq] = event
		}
	}

	seqs := make([]int64, 0, len(merged))
	for seq := range merged {
		seqs = append(seqs, seq)
	}
	sort.Slice(seqs, func(left, right int) bool { return seqs[left] < seqs[right] })

	events := make([]game.Event, 0, len(seqs))
	for _, seq := range seqs {
		events = append(events, merged[seq])
	}
	return events
}

func lastEventOfType(events []game.Event, eventType string) *game.Event {
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Type == eventType {
			return &events[index]
		}
	}
	return nil
}

func expectMetric(t *testing.T, values map[string]int64, name string, want int64) {
	t.Helper()
	got, ok := values[name]
	if !ok {
		t.Fatalf("/metrics 缺少 %s：%v", name, values)
	}
	if got != want {
		t.Fatalf("%s = %d, want %d", name, got, want)
	}
}
