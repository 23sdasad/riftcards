package game

import (
	"fmt"
	"testing"
)

// giveCard 把一张卡牌放进手牌并给足费用，返回实例 ID。
func giveCard(state *State, seat int, cardID string) string {
	definition := state.catalog[cardID]
	instanceID := "test-" + cardID
	state.Players[seat].Hand = append(state.Players[seat].Hand, CardInstance{
		InstanceID: instanceID,
		CardID:     cardID,
		Attack:     definition.Attack,
		Health:     definition.Health,
		MaxHealth:  definition.Health,
	})
	state.Players[seat].Energy = MaxEnergy
	return instanceID
}

// eventsOfType 返回指定类型的事件。
func eventsOfType(events []Event, eventType string) []Event {
	var matched []Event
	for _, event := range events {
		if event.Type == eventType {
			matched = append(matched, event)
		}
	}
	return matched
}

// singleEvent 断言指定类型的事件恰好一条。
func singleEvent(t *testing.T, events []Event, eventType string) Event {
	t.Helper()
	matched := eventsOfType(events, eventType)
	if len(matched) != 1 {
		t.Fatalf("%s 事件数量 = %d, want 1（全部事件：%v）", eventType, len(matched), eventTypes(events))
	}
	return matched[0]
}

// damageEventFor 返回指定目标的 damage_dealt 事件；单位对攻会产生两条，必须按目标区分。
func damageEventFor(t *testing.T, events []Event, targetID string) Event {
	t.Helper()
	for _, event := range eventsOfType(events, "damage_dealt") {
		if event.Data["targetId"] == targetID {
			return event
		}
	}
	t.Fatalf("未找到目标 %s 的 damage_dealt 事件（全部事件：%v）", targetID, eventTypes(events))
	return Event{}
}

// TestSummonEveryUnitCard 逐张核对 6 张单位牌的召唤数值、守卫标记与当回合不能攻击。
func TestSummonEveryUnitCard(t *testing.T) {
	cases := []struct {
		cardID string
		attack int
		health int
		guard  bool
	}{
		{"ember_squire", 2, 1, false},
		{"flame_imp", 3, 1, false},
		{"raider", 4, 2, false},
		{"shield_bearer", 1, 3, true},
		{"stone_sentinel", 2, 5, true},
		{"royal_guard", 3, 6, true},
	}

	for _, testCase := range cases {
		t.Run(testCase.cardID, func(t *testing.T) {
			state := newTestMatch(t, 3)
			seat := state.ActiveSeat
			instanceID := giveCard(state, seat, testCase.cardID)
			revision := state.Revision

			result := state.ApplyCommand(seat, PlayerCommand{Type: CommandPlayCard, CardInstanceID: instanceID})
			if !result.Accepted {
				t.Fatalf("召唤被拒绝：%#v", result.Error)
			}

			board := state.Players[seat].Board
			if len(board) != 1 {
				t.Fatalf("场地单位数 = %d, want 1", len(board))
			}
			unit := board[0]
			if unit.InstanceID != instanceID || unit.CardID != testCase.cardID {
				t.Fatalf("场地单位 = %+v, want 实例 %s", unit, instanceID)
			}
			if unit.Attack != testCase.attack || unit.Health != testCase.health || unit.MaxHealth != testCase.health {
				t.Fatalf("数值 = %d/%d/%d, want %d/%d/%d",
					unit.Attack, unit.Health, unit.MaxHealth, testCase.attack, testCase.health, testCase.health)
			}
			if unit.AttacksRemaining != 0 {
				t.Fatalf("召唤当回合攻击次数 = %d, want 0", unit.AttacksRemaining)
			}

			// 投影中的守卫标记必须与卡牌定义一致。
			view := state.ViewForSeat(seat)
			if view.Players[seat].Board[0].Guard != testCase.guard {
				t.Fatalf("投影守卫 = %v, want %v", view.Players[seat].Board[0].Guard, testCase.guard)
			}

			event := singleEvent(t, result.Events, "unit_summoned")
			if event.Data["guard"] != testCase.guard || event.Data["attack"] != testCase.attack || event.Data["health"] != testCase.health {
				t.Fatalf("unit_summoned 载荷 = %v, want attack=%d health=%d guard=%v",
					event.Data, testCase.attack, testCase.health, testCase.guard)
			}
			if state.Revision != revision+1 {
				t.Fatalf("revision = %d, want %d", state.Revision, revision+1)
			}
		})
	}
}

// TestInsufficientEnergyBlocksEveryCard 8 张牌在费用不足时都必须被拒绝且不改变状态。
func TestInsufficientEnergyBlocksEveryCard(t *testing.T) {
	for cardID, definition := range DefaultCatalog() {
		t.Run(cardID, func(t *testing.T) {
			state := newTestMatch(t, 5)
			seat := state.ActiveSeat
			instanceID := giveCard(state, seat, cardID)
			state.Players[seat].Energy = definition.Cost - 1

			target := ""
			switch cardID {
			case "arcane_bolt":
				target = HeroID(1 - seat)
			case "healing_light":
				target = HeroID(seat)
			}

			revision, seq := state.Revision, state.LastEventSeq
			before := state.ViewForSeat(seat)

			result := state.ApplyCommand(seat, PlayerCommand{
				Type:           CommandPlayCard,
				CardInstanceID: instanceID,
				TargetID:       target,
			})

			if result.Accepted {
				t.Fatal("费用不足的指令不应被接受")
			}
			if result.Error == nil || result.Error.Code != CodeInsufficientEnergy {
				t.Fatalf("错误码 = %#v, want insufficient_energy", result.Error)
			}
			assertNoStateChange(t, state, seat, before, revision, seq)
		})
	}
}

func TestBoardLimitBlocksSummon(t *testing.T) {
	state := newTestMatch(t, 7)
	seat := state.ActiveSeat
	state.Players[seat].Board = make([]CardInstance, 0, BoardLimit)
	for index := 0; index < BoardLimit; index++ {
		state.Players[seat].Board = append(state.Players[seat].Board, CardInstance{
			InstanceID: fmt.Sprintf("u%d", index),
			CardID:     "ember_squire",
			Attack:     2,
			Health:     1,
			MaxHealth:  1,
		})
	}
	instanceID := giveCard(state, seat, "ember_squire")

	revision, seq := state.Revision, state.LastEventSeq
	before := state.ViewForSeat(seat)

	result := state.ApplyCommand(seat, PlayerCommand{Type: CommandPlayCard, CardInstanceID: instanceID})

	if result.Accepted {
		t.Fatal("场地已满时不应接受召唤")
	}
	if result.Error == nil || result.Error.Code != CodeBoardFull {
		t.Fatalf("错误码 = %#v, want board_full", result.Error)
	}
	assertNoStateChange(t, state, seat, before, revision, seq)
}

func TestArcaneBoltDamageAndTargetRules(t *testing.T) {
	t.Run("敌方英雄", func(t *testing.T) {
		state := newTestMatch(t, 9)
		seat := state.ActiveSeat
		opponent := 1 - seat
		instanceID := giveCard(state, seat, "arcane_bolt")
		state.Players[opponent].HP = 30

		result := state.ApplyCommand(seat, PlayerCommand{
			Type:           CommandPlayCard,
			CardInstanceID: instanceID,
			TargetID:       HeroID(opponent),
		})
		if !result.Accepted {
			t.Fatalf("对敌方英雄施放被拒绝：%#v", result.Error)
		}
		if state.Players[opponent].HP != StartingHeroHP-3 {
			t.Fatalf("英雄生命 = %d, want %d", state.Players[opponent].HP, StartingHeroHP-3)
		}
		event := singleEvent(t, result.Events, "damage_dealt")
		if event.Data["amount"] != 3 || event.Data["targetId"] != HeroID(opponent) {
			t.Fatalf("damage_dealt 载荷 = %v, want amount=3 target=%s", event.Data, HeroID(opponent))
		}
	})

	t.Run("任意单位", func(t *testing.T) {
		state := newTestMatch(t, 9)
		seat := state.ActiveSeat
		state.Players[seat].Board = []CardInstance{{
			InstanceID: "own-unit", CardID: "stone_sentinel", Attack: 2, Health: 5, MaxHealth: 5,
		}}
		instanceID := giveCard(state, seat, "arcane_bolt")

		result := state.ApplyCommand(seat, PlayerCommand{
			Type:           CommandPlayCard,
			CardInstanceID: instanceID,
			TargetID:       "own-unit",
		})
		if !result.Accepted {
			t.Fatalf("对己方单位施放被拒绝：%#v", result.Error)
		}
		if state.Players[seat].Board[0].Health != 2 {
			t.Fatalf("单位生命 = %d, want 2", state.Players[seat].Board[0].Health)
		}
	})

	t.Run("不能指定己方英雄", func(t *testing.T) {
		state := newTestMatch(t, 9)
		seat := state.ActiveSeat
		instanceID := giveCard(state, seat, "arcane_bolt")

		revision, seq := state.Revision, state.LastEventSeq
		before := state.ViewForSeat(seat)

		result := state.ApplyCommand(seat, PlayerCommand{
			Type:           CommandPlayCard,
			CardInstanceID: instanceID,
			TargetID:       HeroID(seat),
		})
		if result.Accepted {
			t.Fatal("伤害法术不应能指定己方英雄")
		}
		if result.Error == nil || result.Error.Code != CodeInvalidTarget {
			t.Fatalf("错误码 = %#v, want invalid_target", result.Error)
		}
		assertNoStateChange(t, state, seat, before, revision, seq)
	})

	t.Run("不能指定不存在的目标", func(t *testing.T) {
		state := newTestMatch(t, 9)
		seat := state.ActiveSeat
		instanceID := giveCard(state, seat, "arcane_bolt")

		result := state.ApplyCommand(seat, PlayerCommand{
			Type:           CommandPlayCard,
			CardInstanceID: instanceID,
			TargetID:       "missing-unit",
		})
		if result.Accepted || result.Error == nil || result.Error.Code != CodeInvalidTarget {
			t.Fatalf("结果 = %+v, want invalid_target", result)
		}
	})
}

func TestHealingLightCapsAtMaxAndTargetRules(t *testing.T) {
	t.Run("单位治疗不超过最大生命", func(t *testing.T) {
		state := newTestMatch(t, 13)
		seat := state.ActiveSeat
		state.Players[seat].Board = []CardInstance{{
			InstanceID: "hurt-unit", CardID: "stone_sentinel", Attack: 2, Health: 2, MaxHealth: 5,
		}}
		instanceID := giveCard(state, seat, "healing_light")

		result := state.ApplyCommand(seat, PlayerCommand{
			Type:           CommandPlayCard,
			CardInstanceID: instanceID,
			TargetID:       "hurt-unit",
		})
		if !result.Accepted {
			t.Fatalf("治疗被拒绝：%#v", result.Error)
		}
		if state.Players[seat].Board[0].Health != 5 {
			t.Fatalf("单位生命 = %d, want 5（上限）", state.Players[seat].Board[0].Health)
		}
		event := singleEvent(t, result.Events, "healed")
		if event.Data["amount"] != 3 || event.Data["hpAfter"] != 5 {
			t.Fatalf("healed 载荷 = %v, want amount=3 hpAfter=5", event.Data)
		}
	})

	t.Run("英雄治疗不超过初始生命", func(t *testing.T) {
		state := newTestMatch(t, 13)
		seat := state.ActiveSeat
		state.Players[seat].HP = StartingHeroHP - 1
		instanceID := giveCard(state, seat, "healing_light")

		result := state.ApplyCommand(seat, PlayerCommand{
			Type:           CommandPlayCard,
			CardInstanceID: instanceID,
			TargetID:       HeroID(seat),
		})
		if !result.Accepted {
			t.Fatalf("治疗被拒绝：%#v", result.Error)
		}
		if state.Players[seat].HP != StartingHeroHP {
			t.Fatalf("英雄生命 = %d, want %d", state.Players[seat].HP, StartingHeroHP)
		}
		event := singleEvent(t, result.Events, "healed")
		if event.Data["amount"] != 1 {
			t.Fatalf("healed 载荷 = %v, want amount=1（只恢复差值）", event.Data)
		}
	})

	t.Run("不能指定敌方目标", func(t *testing.T) {
		state := newTestMatch(t, 13)
		seat := state.ActiveSeat
		instanceID := giveCard(state, seat, "healing_light")

		revision, seq := state.Revision, state.LastEventSeq
		before := state.ViewForSeat(seat)

		result := state.ApplyCommand(seat, PlayerCommand{
			Type:           CommandPlayCard,
			CardInstanceID: instanceID,
			TargetID:       HeroID(1 - seat),
		})
		if result.Accepted {
			t.Fatal("治疗不应能指定敌方英雄")
		}
		if result.Error == nil || result.Error.Code != CodeInvalidTarget {
			t.Fatalf("错误码 = %#v, want invalid_target", result.Error)
		}
		assertNoStateChange(t, state, seat, before, revision, seq)
	})
}

func TestUnitCannotAttackTheTurnItIsSummoned(t *testing.T) {
	state := newTestMatch(t, 15)
	seat := state.ActiveSeat
	instanceID := giveCard(state, seat, "raider")

	summon := state.ApplyCommand(seat, PlayerCommand{Type: CommandPlayCard, CardInstanceID: instanceID})
	if !summon.Accepted {
		t.Fatalf("召唤被拒绝：%#v", summon.Error)
	}

	revision, seq := state.Revision, state.LastEventSeq
	before := state.ViewForSeat(seat)

	attack := state.ApplyCommand(seat, PlayerCommand{
		Type:           CommandAttack,
		CardInstanceID: instanceID,
		TargetID:       HeroID(1 - seat),
	})
	if attack.Accepted {
		t.Fatal("召唤当回合不应能攻击")
	}
	if attack.Error == nil || attack.Error.Code != CodeInvalidTarget {
		t.Fatalf("错误码 = %#v, want invalid_target", attack.Error)
	}
	assertNoStateChange(t, state, seat, before, revision, seq)
}

func TestUnitAttacksOncePerTurn(t *testing.T) {
	state := newTestMatch(t, 17)
	seat := state.ActiveSeat
	opponent := 1 - seat
	state.Players[seat].Board = []CardInstance{{
		InstanceID: "attacker", CardID: "raider", Attack: 4, Health: 2, MaxHealth: 2, AttacksRemaining: 1,
	}}

	first := state.ApplyCommand(seat, PlayerCommand{
		Type:           CommandAttack,
		CardInstanceID: "attacker",
		TargetID:       HeroID(opponent),
	})
	if !first.Accepted {
		t.Fatalf("首次攻击被拒绝：%#v", first.Error)
	}
	if state.Players[opponent].HP != StartingHeroHP-4 {
		t.Fatalf("英雄生命 = %d, want %d", state.Players[opponent].HP, StartingHeroHP-4)
	}

	second := state.ApplyCommand(seat, PlayerCommand{
		Type:           CommandAttack,
		CardInstanceID: "attacker",
		TargetID:       HeroID(opponent),
	})
	if second.Accepted {
		t.Fatal("同一回合不应能攻击两次")
	}
	if second.Error == nil || second.Error.Code != CodeInvalidTarget {
		t.Fatalf("错误码 = %#v, want invalid_target", second.Error)
	}
}

// TestGuardAllowsAttackOnGuardItself 守卫存在时，攻击守卫本身是合法的。
func TestGuardAllowsAttackOnGuardItself(t *testing.T) {
	state := newTestMatch(t, 19)
	seat := state.ActiveSeat
	opponent := 1 - seat
	state.Players[seat].Board = []CardInstance{{
		InstanceID: "attacker", CardID: "raider", Attack: 4, Health: 2, MaxHealth: 2, AttacksRemaining: 1,
	}}
	state.Players[opponent].Board = []CardInstance{{
		InstanceID: "guard", CardID: "shield_bearer", Attack: 1, Health: 3, MaxHealth: 3, AttacksRemaining: 1,
	}}

	result := state.ApplyCommand(seat, PlayerCommand{
		Type:           CommandAttack,
		CardInstanceID: "attacker",
		TargetID:       "guard",
	})
	if !result.Accepted {
		t.Fatalf("攻击守卫被拒绝：%#v", result.Error)
	}
	if len(state.Players[opponent].Board) != 0 {
		t.Fatal("生命归零的守卫应离开场地")
	}
	if state.Players[seat].Board[0].Health != 1 {
		t.Fatalf("攻击者生命 = %d, want 1（受到守卫 1 点反击）", state.Players[seat].Board[0].Health)
	}
	damage := damageEventFor(t, result.Events, "guard")
	if damage.Data["amount"] != 4 {
		t.Fatalf("damage_dealt 载荷 = %v, want amount=4", damage.Data)
	}
	died := singleEvent(t, result.Events, "unit_died")
	if died.Data["instanceId"] != "guard" {
		t.Fatalf("unit_died 载荷 = %v, want instanceId=guard", died.Data)
	}
}
