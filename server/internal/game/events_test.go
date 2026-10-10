package game

import (
	"fmt"
	"testing"
)

// TestEventSeqIsContiguousAndRevisionReconciled 固定事件顺序、seq 连续性与 revision 对账。
func TestEventSeqIsContiguousAndRevisionReconciled(t *testing.T) {
	state := newTestMatch(t, 21)
	seatA := state.ActiveSeat
	seatB := 1 - seatA

	var collected []Event
	apply := func(seat int, command PlayerCommand) CommandResult {
		t.Helper()
		revisionBefore := state.Revision
		result := state.ApplyCommand(seat, command)
		if !result.Accepted {
			t.Fatalf("%s 被拒绝：%#v", command.Type, result.Error)
		}
		if state.Revision != revisionBefore+1 {
			t.Fatalf("revision = %d, want %d", state.Revision, revisionBefore+1)
		}
		for _, event := range result.Events {
			if event.Revision != state.Revision {
				t.Fatalf("事件 revision = %d, want %d（同一次结算的事件必须同版本）", event.Revision, state.Revision)
			}
		}
		collected = append(collected, result.Events...)
		return result
	}

	unitA := giveCard(state, seatA, "raider")
	apply(seatA, PlayerCommand{Type: CommandPlayCard, CardInstanceID: unitA})
	apply(seatA, PlayerCommand{Type: CommandEndTurn})

	unitB := giveCard(state, seatB, "ember_squire")
	apply(seatB, PlayerCommand{Type: CommandPlayCard, CardInstanceID: unitB})
	apply(seatB, PlayerCommand{Type: CommandEndTurn})

	apply(seatA, PlayerCommand{Type: CommandAttack, CardInstanceID: unitA, TargetID: HeroID(seatB)})
	apply(seatA, PlayerCommand{Type: CommandEndTurn})

	if len(collected) == 0 {
		t.Fatal("未收集到任何事件")
	}
	for index, event := range collected {
		if event.Seq != int64(index+1) {
			t.Fatalf("第 %d 个事件 seq = %d, want %d（seq 必须从 1 连续递增）", index, event.Seq, index+1)
		}
	}
	if state.LastEventSeq != int64(len(collected)) {
		t.Fatalf("lastEventSeq = %d, want %d", state.LastEventSeq, len(collected))
	}

	// 固定顺序：出牌 -> 召唤；结束回合 -> 对手回合开始 -> 抽牌。
	if got, want := eventTypes(eventsOfType(collected, "card_played")), []string{"card_played", "card_played"}; len(got) != len(want) {
		t.Fatalf("card_played 事件数 = %d, want %d", len(got), len(want))
	}
}

// TestDrawAndBurnVisibilityFollowsSeat 抽牌与烧牌只对该座位可见，公共事件双方可见。
func TestDrawAndBurnVisibilityFollowsSeat(t *testing.T) {
	state := newTestMatch(t, 23)
	seatA := state.ActiveSeat
	seatB := 1 - seatA

	result := state.ApplyCommand(seatA, PlayerCommand{Type: CommandEndTurn})
	if !result.Accepted {
		t.Fatalf("结束回合被拒绝：%#v", result.Error)
	}

	drawn := singleEvent(t, result.Events, "card_drawn")
	if drawn.Visibility != SeatVisibility(seatB) {
		t.Fatalf("card_drawn 可见性 = %s, want %s", drawn.Visibility, SeatVisibility(seatB))
	}
	if ended := singleEvent(t, result.Events, "turn_ended"); ended.Visibility != VisibilityPublic {
		t.Fatalf("turn_ended 可见性 = %s, want public", ended.Visibility)
	}
	if started := singleEvent(t, result.Events, "turn_started"); started.Visibility != VisibilityPublic {
		t.Fatalf("turn_started 可见性 = %s, want public", started.Visibility)
	}

	if got := len(eventsOfType(EventsForSeat(result.Events, seatA), "card_drawn")); got != 0 {
		t.Fatalf("对手抽牌泄露给 %d 个事件", got)
	}
	if got := len(eventsOfType(EventsForSeat(result.Events, seatB), "card_drawn")); got != 1 {
		t.Fatalf("本人应看到自己的抽牌，实际 %d 条", got)
	}
}

// TestCardBurnedWhenHandIsFull 手牌已满时抽到的牌被烧毁，并从牌库移除。
func TestCardBurnedWhenHandIsFull(t *testing.T) {
	state := newTestMatch(t, 25)
	seatA := state.ActiveSeat
	seatB := 1 - seatA

	for len(state.Players[seatA].Hand) < HandLimit {
		state.Players[seatA].Hand = append(state.Players[seatA].Hand, CardInstance{
			InstanceID: fmt.Sprintf("fill-%d", len(state.Players[seatA].Hand)),
			CardID:     "ember_squire",
		})
	}
	handBefore := len(state.Players[seatA].Hand)
	deckBefore := len(state.Players[seatA].Deck)

	if result := state.ApplyCommand(seatA, PlayerCommand{Type: CommandEndTurn}); !result.Accepted {
		t.Fatalf("结束回合被拒绝：%#v", result.Error)
	}
	result := state.ApplyCommand(seatB, PlayerCommand{Type: CommandEndTurn})
	if !result.Accepted {
		t.Fatalf("结束回合被拒绝：%#v", result.Error)
	}

	burned := singleEvent(t, result.Events, "card_burned")
	if burned.Visibility != SeatVisibility(seatA) {
		t.Fatalf("card_burned 可见性 = %s, want %s", burned.Visibility, SeatVisibility(seatA))
	}
	if burned.Data["cardId"] == nil {
		t.Fatalf("card_burned 缺少 cardId：%v", burned.Data)
	}
	if got := len(state.Players[seatA].Hand); got != handBefore {
		t.Fatalf("手牌 = %d, want %d（烧牌不进入手牌）", got, handBefore)
	}
	if got := len(state.Players[seatA].Deck); got != deckBefore-1 {
		t.Fatalf("牌库 = %d, want %d（烧牌仍从牌库移除）", got, deckBefore-1)
	}
}

// TestFatigueDamageIncreasesAndCanEndMatch 牌库为空时按次数递增受疲劳伤害，最终可判负。
func TestFatigueDamageIncreasesAndCanEndMatch(t *testing.T) {
	state := newTestMatch(t, 27)
	for seat := 0; seat < 2; seat++ {
		state.Players[seat].Deck = nil
		state.Players[seat].Hand = nil
	}

	var amounts []int
	for round := 0; round < 40 && state.Status == StatusActive; round++ {
		seat := state.ActiveSeat
		result := state.ApplyCommand(seat, PlayerCommand{Type: CommandEndTurn})
		if !result.Accepted {
			t.Fatalf("结束回合被拒绝：%#v", result.Error)
		}
		for _, event := range eventsOfType(result.Events, "fatigue_damage") {
			amount, ok := event.Data["amount"].(int)
			if !ok {
				t.Fatalf("fatigue_damage 载荷 = %v", event.Data)
			}
			amounts = append(amounts, amount)
		}
	}

	if state.Status != StatusFinished {
		t.Fatalf("status = %s, want finished（疲劳伤害应能结束对局）", state.Status)
	}
	if state.EndReason != EndReasonHeroDefeated {
		t.Fatalf("end reason = %s, want hero_defeated", state.EndReason)
	}

	want := []int{1, 1, 2, 2, 3, 3}
	for index, expected := range want {
		if index >= len(amounts) {
			break
		}
		if amounts[index] != expected {
			t.Fatalf("第 %d 次疲劳伤害 = %d, want %d（每次空抽递增 1）", index, amounts[index], expected)
		}
	}
}

// TestMutualDamageKillsBothUnits 单位对攻致死：双方都产生 unit_died，载荷包含实例 ID。
func TestMutualDamageKillsBothUnits(t *testing.T) {
	state := newTestMatch(t, 29)
	seat := state.ActiveSeat
	opponent := 1 - seat
	state.Players[seat].Board = []CardInstance{{
		InstanceID: "attacker", CardID: "raider", Attack: 4, Health: 2, MaxHealth: 2, AttacksRemaining: 1,
	}}
	state.Players[opponent].Board = []CardInstance{{
		InstanceID: "defender", CardID: "flame_imp", Attack: 3, Health: 1, MaxHealth: 1, AttacksRemaining: 1,
	}}

	result := state.ApplyCommand(seat, PlayerCommand{
		Type:           CommandAttack,
		CardInstanceID: "attacker",
		TargetID:       "defender",
	})
	if !result.Accepted {
		t.Fatalf("攻击被拒绝：%#v", result.Error)
	}

	resolved := singleEvent(t, result.Events, "attack_resolved")
	if resolved.Data["attackerDamage"] != 4 || resolved.Data["defenderDamage"] != 3 {
		t.Fatalf("attack_resolved 载荷 = %v, want attackerDamage=4 defenderDamage=3", resolved.Data)
	}

	damage := eventsOfType(result.Events, "damage_dealt")
	if len(damage) != 2 {
		t.Fatalf("damage_dealt 事件数 = %d, want 2（对攻双方各一条）", len(damage))
	}

	died := eventsOfType(result.Events, "unit_died")
	if len(died) != 2 {
		t.Fatalf("unit_died 事件数 = %d, want 2", len(died))
	}
	ids := map[string]bool{}
	for _, event := range died {
		if event.Data["instanceId"] == nil {
			t.Fatalf("unit_died 缺少 instanceId：%v", event.Data)
		}
		ids[event.Data["instanceId"].(string)] = true
	}
	if !ids["attacker"] || !ids["defender"] {
		t.Fatalf("unit_died 实例 = %v, want attacker 与 defender", ids)
	}
	if len(state.Players[seat].Board) != 0 || len(state.Players[opponent].Board) != 0 {
		t.Fatal("死亡单位应离开场地")
	}
}

// TestProjectionHidesDeckContents 投影不泄露对手手牌内容，牌库只暴露数量。
func TestProjectionHidesDeckContents(t *testing.T) {
	state := newTestMatch(t, 33)
	view := state.ViewForSeat(0)

	if len(view.Players[1].Hand) != 0 {
		t.Fatalf("对手手牌泄露 %d 张", len(view.Players[1].Hand))
	}
	if view.Players[1].HandCount != OpeningHandSize {
		t.Fatalf("对手手牌数 = %d, want %d", view.Players[1].HandCount, OpeningHandSize)
	}
	if view.Players[1].DeckCount != 30-OpeningHandSize {
		t.Fatalf("对手牌库数 = %d, want %d", view.Players[1].DeckCount, 30-OpeningHandSize)
	}
	if len(view.Players[0].Hand) != OpeningHandSize {
		t.Fatalf("本人手牌 = %d, want %d", len(view.Players[0].Hand), OpeningHandSize)
	}
}
