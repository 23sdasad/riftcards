package game

import "testing"

// TestCatalogMatchesGameRules 逐字段核对 8 张 MVP 卡牌与规则文档第 9 节。
func TestCatalogMatchesGameRules(t *testing.T) {
	want := []CardDefinition{
		{ID: "ember_squire", Name: "余烬侍从", Cost: 1, Kind: CardKindUnit, Attack: 2, Health: 1},
		{ID: "flame_imp", Name: "烈焰小鬼", Cost: 2, Kind: CardKindUnit, Attack: 3, Health: 1},
		{ID: "raider", Name: "掠袭者", Cost: 3, Kind: CardKindUnit, Attack: 4, Health: 2},
		{ID: "shield_bearer", Name: "持盾者", Cost: 2, Kind: CardKindUnit, Attack: 1, Health: 3, Guard: true},
		{ID: "stone_sentinel", Name: "石哨兵", Cost: 3, Kind: CardKindUnit, Attack: 2, Health: 5, Guard: true},
		{ID: "royal_guard", Name: "王庭卫兵", Cost: 4, Kind: CardKindUnit, Attack: 3, Health: 6, Guard: true},
		{ID: "arcane_bolt", Name: "奥术飞弹", Cost: 2, Kind: CardKindSpell},
		{ID: "healing_light", Name: "治疗之光", Cost: 2, Kind: CardKindSpell},
	}

	catalog := DefaultCatalog()
	if len(catalog) != len(want) {
		t.Fatalf("卡牌数量 = %d, want %d", len(catalog), len(want))
	}
	for _, expected := range want {
		definition, ok := catalog[expected.ID]
		if !ok {
			t.Fatalf("缺少卡牌 %s", expected.ID)
		}
		if definition.Name != expected.Name {
			t.Fatalf("%s 名称 = %q, want %q", expected.ID, definition.Name, expected.Name)
		}
		if definition.Cost != expected.Cost {
			t.Fatalf("%s 费用 = %d, want %d", expected.ID, definition.Cost, expected.Cost)
		}
		if definition.Kind != expected.Kind {
			t.Fatalf("%s 类型 = %s, want %s", expected.ID, definition.Kind, expected.Kind)
		}
		if definition.Attack != expected.Attack || definition.Health != expected.Health {
			t.Fatalf("%s 数值 = %d/%d, want %d/%d",
				expected.ID, definition.Attack, definition.Health, expected.Attack, expected.Health)
		}
		if definition.Guard != expected.Guard {
			t.Fatalf("%s 守卫 = %v, want %v", expected.ID, definition.Guard, expected.Guard)
		}
	}
}

// TestStarterDeckCompositionMatchesGameRules 核对起始牌组共 30 张且配比与规则一致。
func TestStarterDeckCompositionMatchesGameRules(t *testing.T) {
	want := map[string]int{
		"ember_squire":   4,
		"flame_imp":      3,
		"raider":         4,
		"shield_bearer":  4,
		"stone_sentinel": 3,
		"royal_guard":    2,
		"arcane_bolt":    6,
		"healing_light":  4,
	}
	total := 0
	for _, count := range want {
		total += count
	}
	if total != 30 {
		t.Fatalf("规则配比合计 = %d, want 30", total)
	}

	catalog := DefaultCatalog()
	for seat := 0; seat < 2; seat++ {
		deck := buildStarterDeck(catalog, seat)
		if len(deck) != total {
			t.Fatalf("seat %d 牌组 = %d 张, want %d", seat, len(deck), total)
		}

		counts := map[string]int{}
		instances := map[string]bool{}
		for _, card := range deck {
			counts[card.CardID]++
			if instances[card.InstanceID] {
				t.Fatalf("seat %d 存在重复实例 ID %s", seat, card.InstanceID)
			}
			instances[card.InstanceID] = true

			definition := catalog[card.CardID]
			if card.Attack != definition.Attack || card.Health != definition.Health || card.MaxHealth != definition.Health {
				t.Fatalf("%s 实例数值 = %d/%d/%d, want %d/%d/%d",
					card.CardID, card.Attack, card.Health, card.MaxHealth,
					definition.Attack, definition.Health, definition.Health)
			}
		}
		for cardID, expected := range want {
			if counts[cardID] != expected {
				t.Fatalf("seat %d 的 %s = %d 张, want %d", seat, cardID, counts[cardID], expected)
			}
		}
	}
}
