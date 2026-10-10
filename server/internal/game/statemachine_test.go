package game

import (
	"fmt"
	"slices"
	"testing"
)

// eventTypes 返回事件序列的类型列表，用于断言固定顺序。
func eventTypes(events []Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

// eventSignature 返回包含序号、版本与载荷的事件签名，用于断言回放确定性。
func eventSignature(events []Event) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, fmt.Sprintf("seq=%d rev=%d type=%s actor=%d vis=%s data=%v",
			event.Seq, event.Revision, event.Type, event.ActorSeat, event.Visibility, event.Data))
	}
	return out
}

func TestOpeningPhaseAndTurn(t *testing.T) {
	state := newTestMatch(t, 4)

	if state.Status != StatusActive {
		t.Fatalf("status = %s, want active", state.Status)
	}
	if state.Phase != PhaseAction {
		t.Fatalf("phase = %s, want action", state.Phase)
	}
	if state.Turn != 1 {
		t.Fatalf("turn = %d, want 1", state.Turn)
	}
	if state.ActiveSeat != state.firstSeat {
		t.Fatalf("active seat = %d, want first seat %d", state.ActiveSeat, state.firstSeat)
	}
	if state.EndReason != "" {
		t.Fatalf("end reason = %q, want empty", state.EndReason)
	}
	if state.LastEventSeq != 0 || state.Revision != 0 {
		t.Fatalf("opening seq/revision = %d/%d, want 0/0", state.LastEventSeq, state.Revision)
	}
}

func TestFirstTurnSkipsDrawAndOpponentDraws(t *testing.T) {
	state := newTestMatch(t, 5)
	first := state.ActiveSeat

	result := state.ApplyCommand(first, PlayerCommand{Type: CommandEndTurn})
	if !result.Accepted {
		t.Fatalf("结束回合被拒绝：%#v", result.Error)
	}

	// 固定顺序：结束 -> 对手回合开始 -> 抽牌（后手第一回合正常抽牌）。
	want := []string{"turn_ended", "turn_started", "card_drawn"}
	if got := eventTypes(result.Events); !slices.Equal(got, want) {
		t.Fatalf("事件顺序 = %v, want %v", got, want)
	}
	if state.Turn != 2 {
		t.Fatalf("turn = %d, want 2", state.Turn)
	}
	if state.ActiveSeat != 1-first {
		t.Fatalf("active seat = %d, want %d", state.ActiveSeat, 1-first)
	}
	if state.Phase != PhaseAction {
		t.Fatalf("phase = %s, want action", state.Phase)
	}
	if got := state.Players[state.ActiveSeat].Energy; got != 1 {
		t.Fatalf("新回合费用 = %d, want 1", got)
	}
}

func TestTurnEventOrderIsFixed(t *testing.T) {
	state := newTestMatch(t, 6)
	seat := state.ActiveSeat
	state.Players[seat].Hand = []CardInstance{{InstanceID: "p-card", CardID: "ember_squire"}}
	state.Players[seat].Energy = 1

	play := state.ApplyCommand(seat, PlayerCommand{Type: CommandPlayCard, CardInstanceID: "p-card"})
	if !play.Accepted {
		t.Fatalf("出牌被拒绝：%#v", play.Error)
	}
	if got, want := eventTypes(play.Events), []string{"card_played", "unit_summoned"}; !slices.Equal(got, want) {
		t.Fatalf("出牌事件 = %v, want %v", got, want)
	}

	end := state.ApplyCommand(seat, PlayerCommand{Type: CommandEndTurn})
	if !end.Accepted {
		t.Fatalf("结束回合被拒绝：%#v", end.Error)
	}
	if got, want := eventTypes(end.Events), []string{"turn_ended", "turn_started", "card_drawn"}; !slices.Equal(got, want) {
		t.Fatalf("结束回合事件 = %v, want %v", got, want)
	}
}

func TestIllegalPhaseTransitionDoesNotMutateState(t *testing.T) {
	state := newTestMatch(t, 8)

	// 同阶段与回退迁移都必须被拒绝。
	if err := state.advancePhase(PhaseAction); err == nil {
		t.Fatal("action -> action 应被拒绝")
	}
	if err := state.advancePhase(PhaseStart); err == nil {
		t.Fatal("action -> start 应被拒绝")
	}
	if state.Phase != PhaseAction {
		t.Fatalf("phase = %s, want action（非法迁移不得改变状态）", state.Phase)
	}
	if state.Revision != 0 || state.LastEventSeq != 0 {
		t.Fatalf("非法迁移推进了版本：revision=%d seq=%d", state.Revision, state.LastEventSeq)
	}
}

func TestPhaseTransitionTable(t *testing.T) {
	cases := []struct {
		from, to TurnPhase
		allowed  bool
	}{
		{PhaseIdle, PhaseStart, true},
		{PhaseIdle, PhaseDraw, false},
		{PhaseStart, PhaseDraw, true},
		{PhaseStart, PhaseAction, false},
		{PhaseDraw, PhaseAction, true},
		{PhaseAction, PhaseEnded, true},
		{PhaseEnded, PhaseStart, true},
		{PhaseEnded, PhaseAction, false},
	}
	for _, c := range cases {
		if got := canAdvance(c.from, c.to); got != c.allowed {
			t.Fatalf("canAdvance(%s, %s) = %v, want %v", c.from, c.to, got, c.allowed)
		}
	}

	if !canBeginTurnFrom(PhaseIdle) || !canBeginTurnFrom(PhaseEnded) {
		t.Fatal("idle 与 ended 都应能开始新回合")
	}
	if canBeginTurnFrom(PhaseAction) || canBeginTurnFrom(PhaseStart) {
		t.Fatal("action 与 start 不应能再次开始回合")
	}
}

func TestPlayerCommandsRejectedOutsideActionPhase(t *testing.T) {
	state := newTestMatch(t, 9)
	state.Phase = PhaseDraw // 人为构造非行动阶段

	result := state.ApplyCommand(state.ActiveSeat, PlayerCommand{Type: CommandEndTurn})
	if result.Accepted {
		t.Fatal("非行动阶段的指令不应被接受")
	}
	if result.Error == nil || result.Error.Code != "invalid_command" {
		t.Fatalf("error = %#v, want invalid_command", result.Error)
	}
	if state.Revision != 0 || state.LastEventSeq != 0 || state.Phase != PhaseDraw {
		t.Fatalf("拒绝路径改变了状态：revision=%d seq=%d phase=%s", state.Revision, state.LastEventSeq, state.Phase)
	}

	// 认输不受阶段限制（规则 10）。
	surrender := state.ApplyCommand(1-state.ActiveSeat, PlayerCommand{Type: CommandSurrender})
	if !surrender.Accepted {
		t.Fatalf("非行动阶段的认输应被接受：%#v", surrender.Error)
	}
	if state.Status != StatusFinished || state.EndReason != EndReasonSurrender {
		t.Fatalf("status/reason = %s/%s, want finished/surrender", state.Status, state.EndReason)
	}
}

func TestRejectionDoesNotAdvanceRevisionOrSeq(t *testing.T) {
	state := newTestMatch(t, 10)
	seat := state.ActiveSeat
	state.Players[seat].Hand = nil

	result := state.ApplyCommand(seat, PlayerCommand{Type: CommandPlayCard, CardInstanceID: "missing"})
	if result.Accepted {
		t.Fatal("不存在的手牌不应被接受")
	}
	if result.Error == nil || result.Error.Code != "invalid_card" {
		t.Fatalf("error = %#v, want invalid_card", result.Error)
	}
	if state.Revision != 0 || state.LastEventSeq != 0 {
		t.Fatalf("拒绝路径推进了版本：revision=%d seq=%d", state.Revision, state.LastEventSeq)
	}
	if len(result.Events) != 0 {
		t.Fatalf("拒绝路径产生了 %d 个事件，want 0", len(result.Events))
	}
}

func TestFinishedMatchRejectsEveryCommand(t *testing.T) {
	state := newTestMatch(t, 12)
	seat := state.ActiveSeat

	surrender := state.ApplyCommand(seat, PlayerCommand{Type: CommandSurrender})
	if !surrender.Accepted {
		t.Fatalf("认输被拒绝：%#v", surrender.Error)
	}
	if state.Status != StatusFinished {
		t.Fatalf("status = %s, want finished", state.Status)
	}
	if state.WinnerSeat == nil || *state.WinnerSeat != 1-seat {
		t.Fatalf("winner = %#v, want %d", state.WinnerSeat, 1-seat)
	}

	revision, seq := state.Revision, state.LastEventSeq
	commands := []PlayerCommand{
		{Type: CommandPlayCard, CardInstanceID: "x"},
		{Type: CommandAttack, CardInstanceID: "x", TargetID: "y"},
		{Type: CommandEndTurn},
		{Type: CommandSurrender},
	}
	for _, command := range commands {
		for _, actor := range []int{0, 1} {
			result := state.ApplyCommand(actor, command)
			if result.Accepted {
				t.Fatalf("终局后 %s（seat %d）不应被接受", command.Type, actor)
			}
			if result.Error == nil || result.Error.Code != "match_finished" {
				t.Fatalf("error = %#v, want match_finished", result.Error)
			}
		}
	}
	if state.Revision != revision || state.LastEventSeq != seq {
		t.Fatalf("终局后被推进：revision=%d->%d seq=%d->%d", revision, state.Revision, seq, state.LastEventSeq)
	}
}

func TestEndReasons(t *testing.T) {
	t.Run("surrender", func(t *testing.T) {
		state := newTestMatch(t, 14)
		seat := state.ActiveSeat

		result := state.ApplyCommand(seat, PlayerCommand{Type: CommandSurrender})
		if !result.Accepted {
			t.Fatalf("认输被拒绝：%#v", result.Error)
		}
		if state.EndReason != EndReasonSurrender {
			t.Fatalf("end reason = %s, want surrender", state.EndReason)
		}
		last := result.Events[len(result.Events)-1]
		if last.Type != "match_ended" || last.Data["reason"] != string(EndReasonSurrender) {
			t.Fatalf("结束事件 = %s/%v, want match_ended/surrender", last.Type, last.Data)
		}
	})

	t.Run("hero_defeated", func(t *testing.T) {
		state := newTestMatch(t, 17)
		seat := state.ActiveSeat
		opponent := 1 - seat
		state.Players[seat].Energy = 2
		state.Players[seat].Hand = []CardInstance{{InstanceID: "bolt", CardID: "arcane_bolt"}}
		state.Players[opponent].HP = 3

		result := state.ApplyCommand(seat, PlayerCommand{
			Type:           CommandPlayCard,
			CardInstanceID: "bolt",
			TargetID:       HeroID(opponent),
		})
		if !result.Accepted {
			t.Fatalf("法术被拒绝：%#v", result.Error)
		}
		if state.EndReason != EndReasonHeroDefeated {
			t.Fatalf("end reason = %s, want hero_defeated", state.EndReason)
		}
		if state.WinnerSeat == nil || *state.WinnerSeat != seat {
			t.Fatalf("winner = %#v, want %d", state.WinnerSeat, seat)
		}
		last := result.Events[len(result.Events)-1]
		if last.Type != "match_ended" || last.Data["reason"] != string(EndReasonHeroDefeated) {
			t.Fatalf("结束事件 = %s/%v, want match_ended/hero_defeated", last.Type, last.Data)
		}
	})
}

// replaySignature 用固定种子跑一段固定指令序列，返回完整事件签名。
func replaySignature(t *testing.T, seed int64) []string {
	t.Helper()
	state := newTestMatch(t, seed)
	var out []string

	for turn := 0; turn < 4; turn++ {
		seat := state.ActiveSeat
		if len(state.Players[seat].Hand) > 0 {
			card := state.Players[seat].Hand[0]
			definition := state.catalog[card.CardID]
			canPlay := definition.Kind == CardKindUnit &&
				state.Players[seat].Energy >= definition.Cost &&
				len(state.Players[seat].Board) < BoardLimit
			if canPlay {
				result := state.ApplyCommand(seat, PlayerCommand{Type: CommandPlayCard, CardInstanceID: card.InstanceID})
				if !result.Accepted {
					t.Fatalf("打出单位被拒绝：%#v", result.Error)
				}
				out = append(out, eventSignature(result.Events)...)
			}
		}
		result := state.ApplyCommand(seat, PlayerCommand{Type: CommandEndTurn})
		if !result.Accepted {
			t.Fatalf("结束回合被拒绝：%#v", result.Error)
		}
		out = append(out, eventSignature(result.Events)...)
	}
	return out
}

func TestSameCommandSequenceReplaysIdentically(t *testing.T) {
	first := replaySignature(t, 77)
	second := replaySignature(t, 77)

	if len(first) == 0 {
		t.Fatal("回放序列不应为空")
	}
	if !slices.Equal(first, second) {
		t.Fatalf("同一指令序列产生了不同事件：\n%v\n%v", first, second)
	}
}
