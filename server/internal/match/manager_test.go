package match

import (
	"testing"

	"github.com/mjq/riftcards/server/internal/game"
	"github.com/mjq/riftcards/server/internal/protocol"
)

func TestConnectEchoesRequestIDOnWelcome(t *testing.T) {
	manager := NewManager()
	peer := newFakePeer("peer-0")

	manager.Connect(peer, "req-hello", protocol.HelloData{DisplayName: "Tester"})

	welcome := peer.byType(protocol.TypeWelcome)
	if len(welcome) != 1 {
		t.Fatalf("welcome 数量 = %d, want 1", len(welcome))
	}
	if welcome[0].RequestID != "req-hello" {
		t.Fatalf("requestId = %q, want req-hello", welcome[0].RequestID)
	}
}

func TestJoinQueueEchoesRequestIDWhileWaiting(t *testing.T) {
	manager := NewManager()
	peer := newFakePeer("peer-0")
	manager.Connect(peer, "req-hello", protocol.HelloData{DisplayName: "Tester"})
	peer.reset()

	manager.JoinQueue("peer-0", "req-queue")

	status := peer.byType(protocol.TypeQueueStatus)
	if len(status) != 1 {
		t.Fatalf("queue.status 数量 = %d, want 1", len(status))
	}
	if status[0].RequestID != "req-queue" {
		t.Fatalf("requestId = %q, want req-queue", status[0].RequestID)
	}
	payload, ok := status[0].Data.(protocol.QueueStatusData)
	if !ok || payload.State != "waiting" {
		t.Fatalf("载荷 = %#v, want state=waiting", status[0].Data)
	}
}

func TestLeaveQueueEchoesRequestID(t *testing.T) {
	manager := NewManager()
	peer := newFakePeer("peer-0")
	manager.Connect(peer, "req-hello", protocol.HelloData{DisplayName: "Tester"})
	manager.JoinQueue("peer-0", "req-queue")
	peer.reset()

	manager.LeaveQueue("peer-0", "req-leave")

	status := peer.byType(protocol.TypeQueueStatus)
	if len(status) != 1 {
		t.Fatalf("queue.status 数量 = %d, want 1", len(status))
	}
	if status[0].RequestID != "req-leave" {
		t.Fatalf("requestId = %q, want req-leave", status[0].RequestID)
	}
	payload, ok := status[0].Data.(protocol.QueueStatusData)
	if !ok || payload.State != "left" {
		t.Fatalf("载荷 = %#v, want state=left", status[0].Data)
	}
}

func TestSubmitWithoutRoomReturnsNotInMatch(t *testing.T) {
	manager := NewManager()
	peer := newFakePeer("peer-0")
	manager.Connect(peer, "req-hello", protocol.HelloData{DisplayName: "Tester"})
	peer.reset()

	manager.Submit("peer-0", "req-cmd", protocol.MatchCommandRequest{
		MatchID:   "m_1",
		CommandID: "cmd-1",
		Command:   game.PlayerCommand{Type: game.CommandEndTurn},
	})

	errors := peer.byType(protocol.TypeError)
	if len(errors) != 1 {
		t.Fatalf("未入局提交指令必须显式拒绝，实际错误消息 %d 条", len(errors))
	}
	if errors[0].RequestID != "req-cmd" {
		t.Fatalf("requestId = %q, want req-cmd", errors[0].RequestID)
	}
	payload, ok := errors[0].Data.(protocol.ErrorData)
	if !ok || payload.Code != protocol.CodeNotInMatch {
		t.Fatalf("错误载荷 = %#v, want not_in_match", errors[0].Data)
	}
}

func TestPairingStartsMatchAndRoutesCommands(t *testing.T) {
	manager := NewManager()
	first := newFakePeer("peer-0")
	second := newFakePeer("peer-1")
	manager.Connect(first, "req-hello-0", protocol.HelloData{DisplayName: "One"})
	manager.Connect(second, "req-hello-1", protocol.HelloData{DisplayName: "Two"})

	manager.JoinQueue("peer-0", "req-queue-0")
	manager.JoinQueue("peer-1", "req-queue-1")

	var matchID string
	for _, peer := range []*fakePeer{first, second} {
		started := peer.byType(protocol.TypeMatchStarted)
		if len(started) != 1 {
			t.Fatalf("%s 收到 match.started %d 次, want 1", peer.ID(), len(started))
		}
		if started[0].RequestID != "" {
			t.Fatalf("match.started 是广播，不应回填 requestId，实际 %q", started[0].RequestID)
		}
		payload, ok := started[0].Data.(protocol.MatchStartedData)
		if !ok {
			t.Fatalf("载荷类型 = %T, want MatchStartedData", started[0].Data)
		}
		matchID = payload.MatchID
	}

	// 先入队的一方固定为座位 0，因此指令结果应回到 first。
	manager.Submit("peer-0", "req-cmd", protocol.MatchCommandRequest{
		MatchID:          matchID,
		CommandID:        "cmd-1",
		ExpectedRevision: 0,
		Command:          game.PlayerCommand{Type: game.CommandEndTurn},
	})

	results := first.byType(protocol.TypeMatchCommandDone)
	if len(results) != 1 {
		t.Fatalf("命令结果数量 = %d, want 1", len(results))
	}
	if results[0].RequestID != "req-cmd" {
		t.Fatalf("requestId = %q, want req-cmd", results[0].RequestID)
	}
	result, ok := results[0].Data.(protocol.CommandResultData)
	if !ok || result.CommandID != "cmd-1" {
		t.Fatalf("命令结果 = %#v, want commandId=cmd-1", results[0].Data)
	}
}

func TestDisconnectForfeitsMatch(t *testing.T) {
	manager := NewManager()
	first := newFakePeer("peer-0")
	second := newFakePeer("peer-1")
	manager.Connect(first, "req-hello-0", protocol.HelloData{DisplayName: "One"})
	manager.Connect(second, "req-hello-1", protocol.HelloData{DisplayName: "Two"})
	manager.JoinQueue("peer-0", "req-queue-0")
	manager.JoinQueue("peer-1", "req-queue-1")
	first.reset()
	second.reset()

	manager.Disconnect("peer-0")

	// 断线按认输处理：对手收到带 match_ended 的广播。
	events := second.byType(protocol.TypeMatchEvents)
	if len(events) != 1 {
		t.Fatalf("断线后对手收到广播 %d 条, want 1", len(events))
	}
	payload, ok := events[0].Data.(protocol.MatchEventsData)
	if !ok {
		t.Fatalf("载荷类型 = %T, want MatchEventsData", events[0].Data)
	}
	last := payload.Events[len(payload.Events)-1]
	if last.Type != "match_ended" || last.Data["reason"] != string(game.EndReasonSurrender) {
		t.Fatalf("最后事件 = %s/%v, want match_ended/surrender", last.Type, last.Data)
	}
	if payload.State.WinnerSeat == nil || *payload.State.WinnerSeat != 1 {
		t.Fatalf("winnerSeat = %#v, want 1", payload.State.WinnerSeat)
	}
}
