package game

import "testing"

// newTestMatch 创建对局并断言初始化成功。
func newTestMatch(t *testing.T, seed int64) *State {
	t.Helper()
	state, err := NewMatch("m_test", seed)
	if err != nil {
		t.Fatalf("NewMatch 失败：%v", err)
	}
	return state
}

func TestNewMatchBuildsOpeningState(t *testing.T) {
	state := newTestMatch(t, 42)

	if state.Revision != 0 {
		t.Fatalf("revision = %d, want 0", state.Revision)
	}
	if state.LastEventSeq != 0 {
		t.Fatalf("last event seq = %d, want 0", state.LastEventSeq)
	}
	if state.Players[0].HP != StartingHeroHP || state.Players[1].HP != StartingHeroHP {
		t.Fatalf("hero hp = %d/%d, want %d/%d", state.Players[0].HP, state.Players[1].HP, StartingHeroHP, StartingHeroHP)
	}
	for seat := 0; seat < 2; seat++ {
		if got := len(state.Players[seat].Hand); got != OpeningHandSize {
			t.Fatalf("seat %d hand = %d, want %d", seat, got, OpeningHandSize)
		}
		if got := len(state.Players[seat].Deck); got != 30-OpeningHandSize {
			t.Fatalf("seat %d deck = %d, want %d", seat, got, 30-OpeningHandSize)
		}
	}
	if state.Players[state.ActiveSeat].Energy != 1 {
		t.Fatalf("active energy = %d, want 1", state.Players[state.ActiveSeat].Energy)
	}
}

func TestRejectsCommandFromInactiveSeat(t *testing.T) {
	state := newTestMatch(t, 7)
	inactiveSeat := 1 - state.ActiveSeat

	result := state.ApplyCommand(inactiveSeat, PlayerCommand{Type: CommandEndTurn})

	if result.Accepted {
		t.Fatal("inactive seat command was accepted")
	}
	if result.Error == nil || result.Error.Code != "not_your_turn" {
		t.Fatalf("error = %#v, want not_your_turn", result.Error)
	}
	if state.Revision != 0 {
		t.Fatalf("revision = %d, want 0", state.Revision)
	}
}

func TestPlayingUnitSpendsEnergyAndSummons(t *testing.T) {
	state := newTestMatch(t, 11)
	seat := state.ActiveSeat
	state.Players[seat].Hand = []CardInstance{{
		InstanceID: "p-card",
		CardID:     "ember_squire",
		Attack:     2,
		Health:     1,
		MaxHealth:  1,
	}}
	state.Players[seat].Energy = 1

	result := state.ApplyCommand(seat, PlayerCommand{
		Type:           CommandPlayCard,
		CardInstanceID: "p-card",
	})

	if !result.Accepted {
		t.Fatalf("command rejected: %#v", result.Error)
	}
	if state.Revision != 1 {
		t.Fatalf("revision = %d, want 1", state.Revision)
	}
	if state.Players[seat].Energy != 0 {
		t.Fatalf("energy = %d, want 0", state.Players[seat].Energy)
	}
	if len(state.Players[seat].Hand) != 0 {
		t.Fatalf("hand size = %d, want 0", len(state.Players[seat].Hand))
	}
	if len(state.Players[seat].Board) != 1 {
		t.Fatalf("board size = %d, want 1", len(state.Players[seat].Board))
	}
	if state.Players[seat].Board[0].AttacksRemaining != 0 {
		t.Fatalf("summoned unit attacks = %d, want 0", state.Players[seat].Board[0].AttacksRemaining)
	}
}

func TestAttackMustRespectGuard(t *testing.T) {
	state := newTestMatch(t, 13)
	seat := state.ActiveSeat
	opponent := 1 - seat
	state.Players[seat].Board = []CardInstance{{
		InstanceID:       "attacker",
		CardID:           "raider",
		Attack:           4,
		Health:           2,
		MaxHealth:        2,
		AttacksRemaining: 1,
	}}
	state.Players[opponent].Board = []CardInstance{
		{InstanceID: "guard", CardID: "shield_bearer", Attack: 1, Health: 3, MaxHealth: 3, AttacksRemaining: 1},
		{InstanceID: "target", CardID: "flame_imp", Attack: 3, Health: 1, MaxHealth: 1, AttacksRemaining: 1},
	}

	result := state.ApplyCommand(seat, PlayerCommand{
		Type:           CommandAttack,
		CardInstanceID: "attacker",
		TargetID:       "target",
	})

	if result.Accepted {
		t.Fatal("attack bypassing guard was accepted")
	}
	if result.Error == nil || result.Error.Code != "invalid_target" {
		t.Fatalf("error = %#v, want invalid_target", result.Error)
	}
	if state.Revision != 0 {
		t.Fatalf("revision = %d, want 0", state.Revision)
	}
}

func TestDamageSpellEndsMatch(t *testing.T) {
	state := newTestMatch(t, 17)
	seat := state.ActiveSeat
	opponent := 1 - seat
	state.Players[seat].Energy = 2
	state.Players[seat].Hand = []CardInstance{{
		InstanceID: "bolt",
		CardID:     "arcane_bolt",
	}}
	state.Players[opponent].HP = 3

	result := state.ApplyCommand(seat, PlayerCommand{
		Type:           CommandPlayCard,
		CardInstanceID: "bolt",
		TargetID:       HeroID(opponent),
	})

	if !result.Accepted {
		t.Fatalf("command rejected: %#v", result.Error)
	}
	if state.Status != StatusFinished {
		t.Fatalf("status = %s, want finished", state.Status)
	}
	if state.WinnerSeat == nil || *state.WinnerSeat != seat {
		t.Fatalf("winner = %#v, want %d", state.WinnerSeat, seat)
	}
	if result.Events[len(result.Events)-1].Type != "match_ended" {
		t.Fatalf("last event = %s, want match_ended", result.Events[len(result.Events)-1].Type)
	}
}

func TestWrongPlayerCannotSeeOpponentHand(t *testing.T) {
	state := newTestMatch(t, 23)
	view := state.ViewForSeat(0)

	if len(view.Players[0].Hand) != OpeningHandSize {
		t.Fatalf("viewer hand = %d, want %d", len(view.Players[0].Hand), OpeningHandSize)
	}
	if len(view.Players[1].Hand) != 0 {
		t.Fatalf("opponent hand leaked %d cards", len(view.Players[1].Hand))
	}
	if view.Players[1].HandCount != OpeningHandSize {
		t.Fatalf("opponent hand count = %d, want %d", view.Players[1].HandCount, OpeningHandSize)
	}
}
