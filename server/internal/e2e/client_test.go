package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/mjq/riftcards/server/internal/game"
	"github.com/mjq/riftcards/server/internal/protocol"
)

// readTimeout 是单条消息的读取上限：只是防止测试永久挂住的安全网，
// 不作为同步手段（同步依赖具体消息类型）。
const readTimeout = 60 * time.Second

// commandOutcome 记录一条指令结果，失败时可以定位到请求、修订号和错误码。
type commandOutcome struct {
	RequestID string
	CommandID string
	Command   string
	Accepted  bool
	Revision  int64
	Code      string
}

type serverMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId"`
	Data      json.RawMessage `json:"data"`
}

// testClient 是一个独立的协议客户端：只根据投影决定下一步意图，不推导结算结果。
type testClient struct {
	t    *testing.T
	ctx  context.Context
	conn *websocket.Conn
	name string

	seat    int
	matchID string
	view    *game.MatchView

	revision     int64
	lastEventSeq int64
	// resultRevision 是最近一条指令结果里的版本。
	resultRevision int64
	// awaitingProjection 表示有一条被接受的指令还没在投影里体现。
	// 只有接受路径会广播事件，因此只有它需要等；拒绝路径不推进版本，
	// 若也去等投影就会永久卡住（stale_revision 拒绝不会带来新投影）。
	awaitingProjection bool
	// awaitingResult 表示还有一条指令没有收到响应：同一时刻只允许一条在途指令，
	// 否则响应会与后续请求交错，无法把结果关联回请求。
	awaitingResult bool

	events []game.Event
	seqs   map[int64]bool

	requestCounter    int
	lastSentRequestID string
	pendingCommand    string
	outcomes          []commandOutcome
	eventBatches      int
	probing           bool
	probeResult       *commandOutcome

	projections []game.MatchView
	errors      []string

	attemptedCards   map[string]bool
	attemptedUnits   map[string]bool
	attemptedTargets map[string]bool
	attackedHero     map[string]bool
	endedTurn        bool
	probedOutOfTurn  bool
	accepted         int
	rejected         int
	finished         bool
}

func dialClient(t *testing.T, ctx context.Context, wsURL, name string) *testClient {
	t.Helper()

	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("%s 连接 %s 失败：%v", name, wsURL, err)
	}

	return &testClient{
		t:                t,
		ctx:              ctx,
		conn:             conn,
		name:             name,
		seat:             -1,
		seqs:             map[int64]bool{},
		attemptedCards:   map[string]bool{},
		attemptedUnits:   map[string]bool{},
		attemptedTargets: map[string]bool{},
		attackedHero:     map[string]bool{},
	}
}

// run 完成握手并一直行动到对局结束。
func (c *testClient) run() error {
	c.send(protocol.TypeHello, protocol.HelloData{
		ProtocolVersion: protocol.ProtocolVersion,
		ClientVersion:   "0.1.0",
		DisplayName:     c.name,
	})
	if err := c.waitFor(protocol.TypeWelcome); err != nil {
		return err
	}

	c.send(protocol.TypeQueueJoin, struct{}{})
	if err := c.waitFor(protocol.TypeMatchStarted); err != nil {
		return err
	}

	return c.playLoop()
}

// waitFor 读取消息直到出现指定类型。
func (c *testClient) waitFor(messageType string) error {
	for {
		message, err := c.readMessage()
		if err != nil {
			return err
		}
		if err := c.handle(message); err != nil {
			return err
		}
		if message.Type == messageType {
			return nil
		}
		if c.finished {
			return fmt.Errorf("%s 在对局结束前没有收到 %s", c.name, messageType)
		}
	}
}

// playLoop 每次迭代先尝试推进自己的回合，再读一条服务端消息。
//
// 顺序很重要：match.started 之后服务端不会主动再发消息，如果先阻塞读就会双方互相等待。
// 每次迭代最多提交一条指令，且只在投影追平最近一次指令结果后才继续提交，
// 因此 expectedRevision 始终来自最新投影。
func (c *testClient) playLoop() error {
	for {
		if c.finished {
			return nil
		}

		if c.canAct() {
			if c.probeOutOfTurn() {
				// 越权探测已发出，等它的响应回来再继续正常脚本。
			} else if command, ok := c.nextCommand(); ok {
				c.sendCommand(command)
			}
		}
		c.debugf("canAct=%v awaitingResult=%v awaitingProjection=%v activeSeat=%d seat=%d",
			c.canAct(), c.awaitingResult, c.awaitingProjection, c.activeSeat(), c.seat)

		message, err := c.readMessage()
		if err != nil {
			return err
		}
		if err := c.handle(message); err != nil {
			return err
		}
	}
}

// canAct 判断是否可以在投影基础上继续提交指令。
//
// 两条约束：
//   - 同一时刻只允许一条在途指令（否则响应与请求会交错，无法关联）；
//   - 被接受的指令要等投影追平（它一定会广播 match.events）。拒绝路径不推进版本，
//     若也去等就会永久卡住（stale_revision 拒绝不会带来新投影）。
func (c *testClient) canAct() bool {
	if c.view == nil || c.awaitingResult {
		return false
	}
	if !c.awaitingProjection {
		return true
	}
	if c.view.Revision >= c.resultRevision {
		c.awaitingProjection = false
		return true
	}
	return false
}

func (c *testClient) readMessage() (serverMessage, error) {
	readCtx, cancel := context.WithTimeout(c.ctx, readTimeout)
	defer cancel()

	_, payload, err := c.conn.Read(readCtx)
	if err != nil {
		return serverMessage{}, fmt.Errorf("%s 读取消息失败：%w", c.name, err)
	}

	var message serverMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		return serverMessage{}, fmt.Errorf("%s 解析消息失败：%w（原文 %s）", c.name, err, payload)
	}
	return message, nil
}

func (c *testClient) handle(message serverMessage) error {
	switch message.Type {
	case protocol.TypeWelcome:
		var welcome protocol.WelcomeData
		if err := json.Unmarshal(message.Data, &welcome); err != nil {
			return err
		}
		if welcome.ProtocolVersion != protocol.ProtocolVersion {
			return fmt.Errorf("%s 收到协议版本 %q", c.name, welcome.ProtocolVersion)
		}
		if message.RequestID == "" {
			return fmt.Errorf("%s 的 welcome 缺少 requestId", c.name)
		}

	case protocol.TypeQueueStatus:
		// 排队状态不影响脚本推进。

	case protocol.TypeMatchStarted:
		var started protocol.MatchStartedData
		if err := json.Unmarshal(message.Data, &started); err != nil {
			return err
		}
		c.matchID = started.MatchID
		c.seat = started.Seat
		if err := c.applyView(started.State); err != nil {
			return err
		}

	case protocol.TypeMatchCommandDone:
		var result protocol.CommandResultData
		if err := json.Unmarshal(message.Data, &result); err != nil {
			return err
		}
		if message.RequestID != c.lastSentRequestID {
			return fmt.Errorf("%s 的命令结果 requestId = %q, want %q", c.name, message.RequestID, c.lastSentRequestID)
		}
		outcome := commandOutcome{
			RequestID: message.RequestID,
			CommandID: result.CommandID,
			Command:   c.pendingCommand,
			Accepted:  result.Accepted,
			Revision:  result.Revision,
		}
		if result.Error != nil {
			outcome.Code = result.Error.Code
		}
		c.outcomes = append(c.outcomes, outcome)
		c.resultRevision = result.Revision
		c.awaitingProjection = result.Accepted
		c.awaitingResult = false
		if c.probing {
			c.probing = false
			copied := outcome
			c.probeResult = &copied
		}

		if result.Accepted {
			c.accepted++
			if result.Revision != c.revision+1 {
				return fmt.Errorf("%s 接受路径 revision = %d, want %d（应为前一个版本 +1）",
					c.name, result.Revision, c.revision+1)
			}
		} else {
			c.rejected++
			if result.Error == nil {
				return fmt.Errorf("%s 的拒绝结果缺少错误码（commandId=%s）", c.name, result.CommandID)
			}
			// 拒绝结果里的 revision 是“服务端处理该请求时的当前版本”。两个客户端的
			// 响应流彼此独立：对手的指令可能在本请求之后结算，而它的广播可能先到达，
			// 因此这个值可以落后于客户端已经看到的最新投影（合法竞态，不是状态回退）。
			// 投影单调性由 applyView 断言；“拒绝不推进状态”的强断言在服务端拒绝矩阵测试里。
			if result.Revision < 0 {
				return fmt.Errorf("%s 的拒绝结果带了负版本 %d（code=%s）",
					c.name, result.Revision, result.Error.Code)
			}
			// 被拒绝的结束回合没有生效：允许重试，否则脚本会以为回合已经结束。
			if c.pendingCommand == string(game.CommandEndTurn) {
				c.endedTurn = false
			}
		}
		c.pendingCommand = ""

	case protocol.TypeMatchEvents:
		var payload protocol.MatchEventsData
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			return err
		}
		c.eventBatches++
		if payload.MatchID != c.matchID {
			return fmt.Errorf("%s 收到其它对局的事件：%s", c.name, payload.MatchID)
		}
		if payload.State.Revision != payload.Revision {
			return fmt.Errorf("%s 广播版本不一致：state.revision=%d revision=%d",
				c.name, payload.State.Revision, payload.Revision)
		}
		for _, event := range payload.Events {
			if event.Revision != payload.Revision {
				return fmt.Errorf("%s 事件 revision=%d, want %d（同一次结算必须同版本）",
					c.name, event.Revision, payload.Revision)
			}
			if c.seqs[event.Seq] {
				return fmt.Errorf("%s 收到重复事件 seq=%d", c.name, event.Seq)
			}
			for seq := range c.seqs {
				if seq > event.Seq {
					return fmt.Errorf("%s 事件乱序：已有 seq=%d，又收到 seq=%d", c.name, seq, event.Seq)
				}
			}
			c.seqs[event.Seq] = true
			c.events = append(c.events, event)
		}
		if err := c.applyView(payload.State); err != nil {
			return err
		}

	case protocol.TypeError:
		var protocolErr protocol.ErrorData
		if err := json.Unmarshal(message.Data, &protocolErr); err != nil {
			return err
		}
		c.errors = append(c.errors, protocolErr.Code)
		return fmt.Errorf("%s 收到未预期的 error 响应：%s", c.name, protocolErr.Code)

	case protocol.TypePong:
		// 不参与脚本。

	default:
		return fmt.Errorf("%s 收到未知消息类型 %q", c.name, message.Type)
	}
	return nil
}

// applyView 记录投影并断言隐藏信息与序号单调性。
func (c *testClient) applyView(view game.MatchView) error {
	if view.LastEventSeq < c.lastEventSeq {
		return fmt.Errorf("%s 的 lastEventSeq 回退：%d -> %d", c.name, c.lastEventSeq, view.LastEventSeq)
	}
	if view.Revision < c.revision {
		return fmt.Errorf("%s 的 revision 回退：%d -> %d", c.name, c.revision, view.Revision)
	}

	for _, player := range view.Players {
		if player.Seat == view.YouSeat {
			for _, card := range player.Hand {
				if card.InstanceID == "" || card.CardID == "" {
					return fmt.Errorf("%s 自己的手牌缺少标识：%+v", c.name, card)
				}
			}
			continue
		}
		// 对手只暴露数量，不能出现任何手牌内容。
		if len(player.Hand) != 0 {
			return fmt.Errorf("%s 收到对手手牌内容 %d 张（座位 %d）", c.name, len(player.Hand), player.Seat)
		}
		if player.HandCount < 0 || player.DeckCount < 0 {
			return fmt.Errorf("%s 收到负数数量：hand=%d deck=%d", c.name, player.HandCount, player.DeckCount)
		}
	}

	copyView := view
	c.view = &copyView
	c.revision = view.Revision
	c.lastEventSeq = view.LastEventSeq
	c.projections = append(c.projections, view)
	// 终局快照到达即结束脚本：服务端不会再主动发消息，继续等只会挂住。
	c.finished = view.Status == game.StatusFinished
	return nil
}

func (c *testClient) sendCommand(command game.PlayerCommand) {
	c.requestCounter++
	c.pendingCommand = string(command.Type)
	c.awaitingResult = true
	c.send(protocol.TypeMatchCommand, protocol.MatchCommandRequest{
		MatchID:          c.matchID,
		CommandID:        fmt.Sprintf("%s-cmd-%d", c.name, c.requestCounter),
		ExpectedRevision: c.revision,
		Command:          command,
	})
}

func (c *testClient) send(messageType string, data any) {
	c.requestCounter++
	requestID := fmt.Sprintf("%s-req-%d", c.name, c.requestCounter)
	c.lastSentRequestID = requestID

	envelope := map[string]any{
		"type":      messageType,
		"requestId": requestID,
		"data":      data,
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		c.t.Errorf("%s 编码消息失败：%v", c.name, err)
		return
	}

	writeCtx, cancel := context.WithTimeout(c.ctx, 15*time.Second)
	defer cancel()
	if err := c.conn.Write(writeCtx, websocket.MessageText, payload); err != nil {
		c.t.Errorf("%s 发送 %s 失败：%v", c.name, messageType, err)
	}
}

// nextCommand 依据投影选择下一个意图；返回 false 表示当前无事可做。
//
// 脚本只做“玩家会尝试的事”：出得起的手牌、能攻击的单位、然后结束回合。
// 目标是否合法由服务端裁决；被拒绝的尝试会被记录，并且每个目标只尝试一次。
//
// 回合进度不依赖 Turn 字段的递增方式：只要观察到“不是我的回合”，就重置进度，
// 因此轮到自己时进度一定是从头开始的（否则会出现双方都认为自己无事可做的死锁）。
func (c *testClient) nextCommand() (game.PlayerCommand, bool) {
	if c.view == nil || c.view.Status != "active" {
		return game.PlayerCommand{}, false
	}
	if c.view.ActiveSeat != c.seat {
		c.resetTurnProgress()
		return game.PlayerCommand{}, false
	}

	you := c.player(c.seat)
	opponent := c.player(1 - c.seat)
	if you == nil || opponent == nil {
		return game.PlayerCommand{}, false
	}

	// 1) 出牌：先单位，后法术（法术尝试指向对手英雄）。
	for _, card := range you.Hand {
		if c.attemptedCards[card.InstanceID] || card.Cost > you.Energy {
			continue
		}
		c.attemptedCards[card.InstanceID] = true
		if card.Kind == game.CardKindSpell {
			return game.PlayerCommand{
				Type:           game.CommandPlayCard,
				CardInstanceID: card.InstanceID,
				TargetID:       opponent.HeroID,
			}, true
		}
		return game.PlayerCommand{Type: game.CommandPlayCard, CardInstanceID: card.InstanceID}, true
	}

	// 2) 攻击：先试对手英雄，被拒后再逐个试对手单位。
	for _, unit := range you.Board {
		if c.attemptedUnits[unit.InstanceID] || unit.AttacksRemaining <= 0 {
			continue
		}
		if !c.attackedHero[unit.InstanceID] {
			c.attackedHero[unit.InstanceID] = true
			return game.PlayerCommand{
				Type:           game.CommandAttack,
				CardInstanceID: unit.InstanceID,
				TargetID:       opponent.HeroID,
			}, true
		}
		for _, target := range opponent.Board {
			key := unit.InstanceID + "|" + target.InstanceID
			if c.attemptedTargets[key] {
				continue
			}
			c.attemptedTargets[key] = true
			return game.PlayerCommand{
				Type:           game.CommandAttack,
				CardInstanceID: unit.InstanceID,
				TargetID:       target.InstanceID,
			}, true
		}
		c.attemptedUnits[unit.InstanceID] = true
	}

	// 3) 结束回合。
	if !c.endedTurn {
		c.endedTurn = true
		return game.PlayerCommand{Type: game.CommandEndTurn}, true
	}
	return game.PlayerCommand{}, false
}

// probeOutOfTurn 在自己的回合之外故意提交一条指令，用于端到端验证拒绝路径。
// 每个客户端只做一次；返回是否已发出指令。
func (c *testClient) probeOutOfTurn() bool {
	if c.probedOutOfTurn || c.view == nil || c.view.Status != "active" || c.view.ActiveSeat == c.seat {
		return false
	}
	c.probedOutOfTurn = true
	c.probing = true
	c.sendCommand(game.PlayerCommand{Type: game.CommandEndTurn})
	return true
}

// resetTurnProgress 清空本回合的尝试记录，让下一次轮到自己时重新开始。
func (c *testClient) resetTurnProgress() {
	c.attemptedCards = map[string]bool{}
	c.attemptedUnits = map[string]bool{}
	c.attemptedTargets = map[string]bool{}
	c.attackedHero = map[string]bool{}
	c.endedTurn = false
}

func (c *testClient) player(seat int) *game.PlayerView {
	if c.view == nil {
		return nil
	}
	for index := range c.view.Players {
		if c.view.Players[index].Seat == seat {
			return &c.view.Players[index]
		}
	}
	return nil
}

func (c *testClient) close() {
	_ = c.conn.Close(websocket.StatusNormalClosure, "test finished")
}

// activeSeat 返回当前投影里的行动座位；没有投影时返回 -1。
func (c *testClient) activeSeat() int {
	if c.view == nil {
		return -1
	}
	return c.view.ActiveSeat
}

// debugf 在 E2E_DEBUG 非空时输出诊断行，默认静默。
func (c *testClient) debugf(format string, args ...any) {
	if os.Getenv("E2E_DEBUG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "[e2e] %s "+format+"\n", append([]any{c.name}, args...)...)
}

// diagnostics 汇总关键状态，失败时用于定位。
func (c *testClient) diagnostics() string {
	text := fmt.Sprintf("%s: seat=%d match=%s revision=%d lastEventSeq=%d events=%d accepted=%d rejected=%d projections=%d errors=%v",
		c.name, c.seat, c.matchID, c.revision, c.lastEventSeq, len(c.events), c.accepted, c.rejected, len(c.projections), c.errors)
	if c.view != nil {
		text += fmt.Sprintf("\n  view: turn=%d activeSeat=%d status=%s revision=%d lastEventSeq=%d",
			c.view.Turn, c.view.ActiveSeat, c.view.Status, c.view.Revision, c.view.LastEventSeq)
	}
	if you := c.player(c.seat); you != nil {
		text += fmt.Sprintf("\n  you: hp=%d energy=%d hand=%d board=%d",
			you.HP, you.Energy, len(you.Hand), len(you.Board))
		for _, card := range you.Hand {
			text += fmt.Sprintf("\n    card=%s cost=%d kind=%s attempted=%v",
				card.CardID, card.Cost, card.Kind, c.attemptedCards[card.InstanceID])
		}
		for _, unit := range you.Board {
			text += fmt.Sprintf("\n    unit=%s %d/%d attacks=%d attackedHero=%v attempted=%v",
				unit.CardID, unit.Attack, unit.Health, unit.AttacksRemaining,
				c.attackedHero[unit.InstanceID], c.attemptedUnits[unit.InstanceID])
		}
	}
	text += fmt.Sprintf("\n  bookkeeping: endedTurn=%v awaitingProjection=%v probed=%v",
		c.endedTurn, c.awaitingProjection, c.probedOutOfTurn)
	for _, outcome := range c.outcomes {
		text += fmt.Sprintf("\n  requestId=%s commandId=%s command=%s accepted=%v revision=%d code=%s",
			outcome.RequestID, outcome.CommandID, outcome.Command, outcome.Accepted, outcome.Revision, outcome.Code)
	}
	return text
}
