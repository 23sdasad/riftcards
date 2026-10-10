package game

import (
	"reflect"
	"testing"
)

// assertNoStateChange 断言拒绝路径没有改变权威状态、revision、seq 或待发事件。
func assertNoStateChange(t *testing.T, state *State, seat int, before MatchView, revision, seq int64) {
	t.Helper()
	if state.Revision != revision {
		t.Fatalf("revision = %d, want %d", state.Revision, revision)
	}
	if state.LastEventSeq != seq {
		t.Fatalf("lastEventSeq = %d, want %d", state.LastEventSeq, seq)
	}
	if len(state.pendingEvents) != 0 {
		t.Fatalf("拒绝路径留下了 %d 个待发事件", len(state.pendingEvents))
	}
	after := state.ViewForSeat(seat)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("拒绝路径改变了投影：\nbefore=%+v\nafter=%+v", before, after)
	}
}

// TestRejectionMatrixLeavesStateUnchanged 逐个错误码验证拒绝路径不改变任何权威状态。
func TestRejectionMatrixLeavesStateUnchanged(t *testing.T) {
	cases := []struct {
		name  string
		code  string
		setup func(t *testing.T, state *State) (int, PlayerCommand)
	}{
		{
			name: "invalid_card",
			code: CodeInvalidCard,
			setup: func(_ *testing.T, state *State) (int, PlayerCommand) {
				return state.ActiveSeat, PlayerCommand{Type: CommandPlayCard, CardInstanceID: "missing-card"}
			},
		},
		{
			name: "insufficient_energy",
			code: CodeInsufficientEnergy,
			setup: func(_ *testing.T, state *State) (int, PlayerCommand) {
				seat := state.ActiveSeat
				instanceID := giveCard(state, seat, "raider")
				state.Players[seat].Energy = 0
				return seat, PlayerCommand{Type: CommandPlayCard, CardInstanceID: instanceID}
			},
		},
		{
			name: "invalid_target",
			code: CodeInvalidTarget,
			setup: func(_ *testing.T, state *State) (int, PlayerCommand) {
				seat := state.ActiveSeat
				instanceID := giveCard(state, seat, "arcane_bolt")
				return seat, PlayerCommand{
					Type:           CommandPlayCard,
					CardInstanceID: instanceID,
					TargetID:       HeroID(seat),
				}
			},
		},
		{
			name: "board_full",
			code: CodeBoardFull,
			setup: func(_ *testing.T, state *State) (int, PlayerCommand) {
				seat := state.ActiveSeat
				for index := 0; index < BoardLimit; index++ {
					state.Players[seat].Board = append(state.Players[seat].Board, CardInstance{
						InstanceID: "unit-" + string(rune('a'+index)),
						CardID:     "ember_squire",
						Attack:     2,
						Health:     1,
						MaxHealth:  1,
					})
				}
				instanceID := giveCard(state, seat, "ember_squire")
				return seat, PlayerCommand{Type: CommandPlayCard, CardInstanceID: instanceID}
			},
		},
		{
			name: "not_your_turn",
			code: CodeNotYourTurn,
			setup: func(_ *testing.T, state *State) (int, PlayerCommand) {
				return 1 - state.ActiveSeat, PlayerCommand{Type: CommandEndTurn}
			},
		},
		{
			name: "invalid_command",
			code: CodeInvalidCommand,
			setup: func(_ *testing.T, state *State) (int, PlayerCommand) {
				return state.ActiveSeat, PlayerCommand{Type: CommandType("unknown_command")}
			},
		},
		{
			name: "match_finished",
			code: CodeMatchFinished,
			setup: func(t *testing.T, state *State) (int, PlayerCommand) {
				t.Helper()
				seat := state.ActiveSeat
				if result := state.ApplyCommand(seat, PlayerCommand{Type: CommandSurrender}); !result.Accepted {
					t.Fatalf("认输被拒绝：%#v", result.Error)
				}
				return seat, PlayerCommand{Type: CommandEndTurn}
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			state := newTestMatch(t, 31)
			seat, command := testCase.setup(t, state)

			revision, seq := state.Revision, state.LastEventSeq
			before := state.ViewForSeat(seat)

			result := state.ApplyCommand(seat, command)

			if result.Accepted {
				t.Fatalf("%s 不应被接受", testCase.name)
			}
			if result.Error == nil || result.Error.Code != testCase.code {
				t.Fatalf("错误码 = %#v, want %s", result.Error, testCase.code)
			}
			if len(result.Events) != 0 {
				t.Fatalf("拒绝路径产生了 %d 个事件", len(result.Events))
			}
			assertNoStateChange(t, state, seat, before, revision, seq)
		})
	}
}
