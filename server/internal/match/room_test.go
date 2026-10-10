package match

import (
	"testing"

	"github.com/mjq/riftcards/server/internal/game"
	"github.com/mjq/riftcards/server/internal/protocol"
)

// newTestRoom 创建一个已开始的对局房间，并返回房间与两个假连接。
func newTestRoom(t *testing.T, matchID string) (*Room, *fakePeer, *fakePeer) {
	t.Helper()
	first := newFakePeer("peer-0")
	second := newFakePeer("peer-1")
	room, err := NewRoom(matchID, 42, [2]Peer{first, second})
	if err != nil {
		t.Fatalf("NewRoom 失败：%v", err)
	}
	room.Start()
	first.reset()
	second.reset()
	return room, first, second
}

// commandResult 从消息载荷中解出命令结果。
func commandResult(t *testing.T, message protocol.ServerEnvelope) protocol.CommandResultData {
	t.Helper()
	result, ok := message.Data.(protocol.CommandResultData)
	if !ok {
		t.Fatalf("载荷类型 = %T, want CommandResultData", message.Data)
	}
	return result
}

func matchEvents(t *testing.T, message protocol.ServerEnvelope) protocol.MatchEventsData {
	t.Helper()
	events, ok := message.Data.(protocol.MatchEventsData)
	if !ok {
		t.Fatalf("载荷类型 = %T, want MatchEventsData", message.Data)
	}
	return events
}

func TestSubmitEchoesRequestIDAndSharesSettlementRevision(t *testing.T) {
	room, first, second := newTestRoom(t, "m_1")
	seat := room.state.ActiveSeat
	baseRevision := room.state.Revision

	room.Submit(seat, "req-cmd-1", protocol.MatchCommandRequest{
		MatchID:          "m_1",
		CommandID:        "cmd-1",
		ExpectedRevision: baseRevision,
		Command:          game.PlayerCommand{Type: game.CommandEndTurn},
	})

	peer := first
	if seat == 1 {
		peer = second
	}
	results := peer.byType(protocol.TypeMatchCommandDone)
	if len(results) != 1 {
		t.Fatalf("命令结果数量 = %d, want 1", len(results))
	}
	if results[0].RequestID != "req-cmd-1" {
		t.Fatalf("requestId = %q, want req-cmd-1", results[0].RequestID)
	}
	result := commandResult(t, results[0])
	if !result.Accepted || result.CommandID != "cmd-1" {
		t.Fatalf("命令结果 = %+v, want accepted 且 commandId=cmd-1", result)
	}
	if result.Revision <= baseRevision {
		t.Fatalf("revision = %d, want > %d", result.Revision, baseRevision)
	}

	// 事件与投影必须来自同一次结算：baseRevision 是结算前版本，revision 是结算后版本。
	for _, eventPeer := range []*fakePeer{first, second} {
		events := eventPeer.byType(protocol.TypeMatchEvents)
		if len(events) != 1 {
			t.Fatalf("广播数量 = %d, want 1", len(events))
		}
		if events[0].RequestID != "" {
			t.Fatalf("广播不应回填 requestId，实际 %q", events[0].RequestID)
		}
		payload := matchEvents(t, events[0])
		if payload.BaseRevision != baseRevision {
			t.Fatalf("baseRevision = %d, want %d", payload.BaseRevision, baseRevision)
		}
		if payload.Revision != result.Revision {
			t.Fatalf("广播 revision = %d, want %d", payload.Revision, result.Revision)
		}
		if payload.State.Revision != result.Revision {
			t.Fatalf("投影 revision = %d, want %d", payload.State.Revision, result.Revision)
		}
		if payload.LastEventSeq != room.state.LastEventSeq {
			t.Fatalf("lastEventSeq = %d, want %d", payload.LastEventSeq, room.state.LastEventSeq)
		}
		if len(payload.Events) == 0 {
			t.Fatal("接受路径应广播事件")
		}
	}
}

func TestSubmitRejectsStaleRevisionWithoutSideEffects(t *testing.T) {
	room, first, second := newTestRoom(t, "m_1")
	seat := room.state.ActiveSeat
	revision := room.state.Revision
	seq := room.state.LastEventSeq

	room.Submit(seat, "req-stale", protocol.MatchCommandRequest{
		MatchID:          "m_1",
		CommandID:        "cmd-stale",
		ExpectedRevision: revision + 5,
		Command:          game.PlayerCommand{Type: game.CommandEndTurn},
	})

	peer := first
	if seat == 1 {
		peer = second
	}
	results := peer.byType(protocol.TypeMatchCommandDone)
	if len(results) != 1 {
		t.Fatalf("命令结果数量 = %d, want 1", len(results))
	}
	if results[0].RequestID != "req-stale" {
		t.Fatalf("requestId = %q, want req-stale", results[0].RequestID)
	}
	result := commandResult(t, results[0])
	if result.Accepted || result.Error == nil || result.Error.Code != protocol.CodeStaleRevision {
		t.Fatalf("命令结果 = %+v, want stale_revision", result)
	}
	if result.Revision != revision {
		t.Fatalf("响应 revision = %d, want 未变化的 %d", result.Revision, revision)
	}
	if room.state.Revision != revision || room.state.LastEventSeq != seq {
		t.Fatalf("版本冲突改变了状态：revision=%d seq=%d", room.state.Revision, room.state.LastEventSeq)
	}
	if len(first.byType(protocol.TypeMatchEvents)) != 0 || len(second.byType(protocol.TypeMatchEvents)) != 0 {
		t.Fatal("版本冲突不应广播事件")
	}
}

func TestSubmitRejectsForeignMatchID(t *testing.T) {
	room, first, second := newTestRoom(t, "m_1")
	seat := room.state.ActiveSeat
	revision := room.state.Revision

	room.Submit(seat, "req-foreign", protocol.MatchCommandRequest{
		MatchID:          "m_other",
		CommandID:        "cmd-foreign",
		ExpectedRevision: revision,
		Command:          game.PlayerCommand{Type: game.CommandEndTurn},
	})

	peer := first
	if seat == 1 {
		peer = second
	}
	errors := peer.byType(protocol.TypeError)
	if len(errors) != 1 {
		t.Fatalf("错误消息数量 = %d, want 1", len(errors))
	}
	if errors[0].RequestID != "req-foreign" {
		t.Fatalf("requestId = %q, want req-foreign", errors[0].RequestID)
	}
	payload, ok := errors[0].Data.(protocol.ErrorData)
	if !ok || payload.Code != protocol.CodeNotInMatch {
		t.Fatalf("错误载荷 = %#v, want not_in_match", errors[0].Data)
	}
	if room.state.Revision != revision {
		t.Fatalf("revision = %d, want %d", room.state.Revision, revision)
	}
}

func TestSubmitRejectsCommandAfterMatchFinished(t *testing.T) {
	room, first, second := newTestRoom(t, "m_1")
	seat := room.state.ActiveSeat

	// 先认输结束对局。
	room.Forfeit(seat)
	if room.state.Status != game.StatusFinished {
		t.Fatalf("status = %s, want finished", room.state.Status)
	}
	revision := room.state.Revision
	seq := room.state.LastEventSeq
	first.reset()
	second.reset()

	active := room.state.ActiveSeat
	room.Submit(active, "req-after-end", protocol.MatchCommandRequest{
		MatchID:          "m_1",
		CommandID:        "cmd-after-end",
		ExpectedRevision: revision,
		Command:          game.PlayerCommand{Type: game.CommandEndTurn},
	})

	peer := first
	if active == 1 {
		peer = second
	}
	results := peer.byType(protocol.TypeMatchCommandDone)
	if len(results) != 1 {
		t.Fatalf("命令结果数量 = %d, want 1", len(results))
	}
	if results[0].RequestID != "req-after-end" {
		t.Fatalf("requestId = %q, want req-after-end", results[0].RequestID)
	}
	result := commandResult(t, results[0])
	if result.Accepted || result.Error == nil || result.Error.Code != game.CodeMatchFinished {
		t.Fatalf("命令结果 = %+v, want match_finished", result)
	}
	if room.state.Revision != revision || room.state.LastEventSeq != seq {
		t.Fatalf("终局后指令改变了状态：revision=%d seq=%d", room.state.Revision, room.state.LastEventSeq)
	}
	if len(first.byType(protocol.TypeMatchEvents)) != 0 || len(second.byType(protocol.TypeMatchEvents)) != 0 {
		t.Fatal("终局后指令不应广播事件")
	}
}

func TestSubmitSerializesCommandsAcrossTurns(t *testing.T) {
	room, first, second := newTestRoom(t, "m_1")

	previousRevision := room.state.Revision
	for turn := 0; turn < 4; turn++ {
		seat := room.state.ActiveSeat
		room.Submit(seat, "req-turn", protocol.MatchCommandRequest{
			MatchID:          "m_1",
			CommandID:        "cmd-turn",
			ExpectedRevision: room.state.Revision,
			Command:          game.PlayerCommand{Type: game.CommandEndTurn},
		})

		peer := first
		if seat == 1 {
			peer = second
		}
		events := peer.byType(protocol.TypeMatchEvents)
		if len(events) == 0 {
			t.Fatalf("第 %d 回合没有广播", turn)
		}
		payload := matchEvents(t, events[len(events)-1])
		if payload.BaseRevision != previousRevision {
			t.Fatalf("第 %d 回合 baseRevision = %d, want %d", turn, payload.BaseRevision, previousRevision)
		}
		if payload.Revision <= previousRevision {
			t.Fatalf("第 %d 回合 revision = %d, want > %d", turn, payload.Revision, previousRevision)
		}
		if payload.State.Revision != payload.Revision {
			t.Fatalf("第 %d 回合投影与广播版本不一致：%d vs %d", turn, payload.State.Revision, payload.Revision)
		}
		previousRevision = payload.Revision
		first.reset()
		second.reset()
	}
}

func TestForfeitBroadcastsTypedEventsWithEndReason(t *testing.T) {
	room, first, second := newTestRoom(t, "m_1")
	seat := room.state.ActiveSeat

	room.Forfeit(seat)

	for _, peer := range []*fakePeer{first, second} {
		events := peer.byType(protocol.TypeMatchEvents)
		if len(events) != 1 {
			t.Fatalf("广播数量 = %d, want 1", len(events))
		}
		payload := matchEvents(t, events[0])
		if payload.MatchID != "m_1" {
			t.Fatalf("matchId = %q, want m_1", payload.MatchID)
		}
		last := payload.Events[len(payload.Events)-1]
		if last.Type != "match_ended" || last.Data["reason"] != string(game.EndReasonSurrender) {
			t.Fatalf("最后事件 = %s/%v, want match_ended/surrender", last.Type, last.Data)
		}
		if payload.State.Status != game.StatusFinished {
			t.Fatalf("投影状态 = %s, want finished", payload.State.Status)
		}
	}
}

func TestForfeitIsIdempotent(t *testing.T) {
	room, first, second := newTestRoom(t, "m_1")
	seat := room.state.ActiveSeat

	room.Forfeit(seat)
	revision := room.state.Revision
	seq := room.state.LastEventSeq
	first.reset()
	second.reset()

	room.Forfeit(1 - seat)

	if room.state.Revision != revision || room.state.LastEventSeq != seq {
		t.Fatalf("重复认输改变了状态：revision=%d seq=%d", room.state.Revision, room.state.LastEventSeq)
	}
	if len(first.byType(protocol.TypeMatchEvents)) != 0 || len(second.byType(protocol.TypeMatchEvents)) != 0 {
		t.Fatal("重复认输不应再次广播")
	}
}

func TestReplayEventsKeepsAcceptedEventsOnly(t *testing.T) {
	room, _, _ := newTestRoom(t, "m_1")
	seat := room.state.ActiveSeat

	room.Submit(seat, "req-ok", protocol.MatchCommandRequest{
		MatchID:          "m_1",
		CommandID:        "cmd-ok",
		ExpectedRevision: room.state.Revision,
		Command:          game.PlayerCommand{Type: game.CommandEndTurn},
	})
	accepted := len(room.ReplayEvents())
	if accepted == 0 {
		t.Fatal("接受路径应写入事件日志")
	}

	room.Submit(seat, "req-stale", protocol.MatchCommandRequest{
		MatchID:          "m_1",
		CommandID:        "cmd-stale",
		ExpectedRevision: room.state.Revision + 1,
		Command:          game.PlayerCommand{Type: game.CommandEndTurn},
	})
	if got := len(room.ReplayEvents()); got != accepted {
		t.Fatalf("拒绝路径写入了事件日志：%d -> %d", accepted, got)
	}
}
