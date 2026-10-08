package game

import "fmt"

func DefaultCatalog() map[string]CardDefinition {
	definitions := []CardDefinition{
		{ID: "ember_squire", Name: "余烬侍从", Cost: 1, Kind: CardKindUnit, Attack: 2, Health: 1},
		{ID: "flame_imp", Name: "烈焰小鬼", Cost: 2, Kind: CardKindUnit, Attack: 3, Health: 1},
		{ID: "raider", Name: "掠袭者", Cost: 3, Kind: CardKindUnit, Attack: 4, Health: 2},
		{ID: "shield_bearer", Name: "持盾者", Cost: 2, Kind: CardKindUnit, Attack: 1, Health: 3, Guard: true},
		{ID: "stone_sentinel", Name: "石哨兵", Cost: 3, Kind: CardKindUnit, Attack: 2, Health: 5, Guard: true},
		{ID: "royal_guard", Name: "王庭卫兵", Cost: 4, Kind: CardKindUnit, Attack: 3, Health: 6, Guard: true},
		{ID: "arcane_bolt", Name: "奥术飞弹", Cost: 2, Kind: CardKindSpell, Text: "对目标造成 3 点伤害"},
		{ID: "healing_light", Name: "治疗之光", Cost: 2, Kind: CardKindSpell, Text: "为目标恢复 5 点生命"},
	}

	catalog := make(map[string]CardDefinition, len(definitions))
	for _, definition := range definitions {
		catalog[definition.ID] = definition
	}
	return catalog
}

func buildStarterDeck(catalog map[string]CardDefinition, seat int) []CardInstance {
	counts := map[string]int{
		"ember_squire":   4,
		"flame_imp":      3,
		"raider":         4,
		"shield_bearer":  4,
		"stone_sentinel": 3,
		"royal_guard":    2,
		"arcane_bolt":    6,
		"healing_light":  4,
	}

	deck := make([]CardInstance, 0, 30)
	instanceNumber := 0
	for _, cardID := range []string{
		"ember_squire",
		"flame_imp",
		"raider",
		"shield_bearer",
		"stone_sentinel",
		"royal_guard",
		"arcane_bolt",
		"healing_light",
	} {
		definition, ok := catalog[cardID]
		if !ok {
			panic(fmt.Sprintf("starter deck references unknown card %q", cardID))
		}
		for range counts[cardID] {
			deck = append(deck, CardInstance{
				InstanceID: fmt.Sprintf("p%d-c%d", seat, instanceNumber),
				CardID:     cardID,
				Attack:     definition.Attack,
				Health:     definition.Health,
				MaxHealth:  definition.Health,
			})
			instanceNumber++
		}
	}
	return deck
}
