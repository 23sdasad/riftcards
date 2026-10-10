package game

import (
	"fmt"
	"math/rand"
)

const (
	StartingHeroHP  = 30
	OpeningHandSize = 4
	HandLimit       = 8
	BoardLimit      = 5
	MaxEnergy       = 10
)

func HeroID(seat int) string {
	return fmt.Sprintf("hero-%d", seat)
}

// NewMatch 创建对局并进入第一个回合的行动阶段。
//
// 开局发牌不产生事件（emit=false），因此返回时 LastEventSeq 仍为 0、Revision 为 0。
// 阶段推进经由 beginTurn，与后续回合共用同一条迁移路径。
func NewMatch(matchID string, seed int64) (*State, error) {
	state := &State{
		MatchID:   matchID,
		Status:    StatusActive,
		Phase:     PhaseIdle,
		catalog:   DefaultCatalog(),
		rng:       rand.New(rand.NewSource(seed)),
		Players:   [2]PlayerState{},
		firstSeat: 0,
	}

	for seat := 0; seat < 2; seat++ {
		player := &state.Players[seat]
		player.Seat = seat
		player.HeroID = HeroID(seat)
		player.HP = StartingHeroHP
		player.Deck = buildStarterDeck(state.catalog, seat)
		state.rng.Shuffle(len(player.Deck), func(i, j int) {
			player.Deck[i], player.Deck[j] = player.Deck[j], player.Deck[i]
		})
		state.draw(seat, OpeningHandSize, false)
	}

	state.firstSeat = state.rng.Intn(2)
	if err := state.beginTurn(state.firstSeat, false); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *State) ApplyCommand(seat int, command PlayerCommand) CommandResult {
	if s.Status != StatusActive {
		return s.reject("match_finished", "match is already finished")
	}

	// 认输在任意阶段都允许（规则 10）。
	if command.Type == CommandSurrender {
		return s.surrender(seat)
	}

	// 除认输外，玩家指令只在行动阶段被接受。正常流程下阶段总是 action，
	// 这里是状态机的前置守卫：拒绝时不修改任何状态。
	if s.Phase != PhaseAction {
		return s.reject("invalid_command", fmt.Sprintf("当前回合阶段 %s 不接受玩家指令", s.Phase))
	}

	if seat != s.ActiveSeat {
		return s.reject("not_your_turn", "it is not your turn")
	}

	switch command.Type {
	case CommandPlayCard:
		return s.playCard(seat, command)
	case CommandAttack:
		return s.attack(seat, command)
	case CommandEndTurn:
		return s.endTurn(seat)
	default:
		return s.reject("invalid_command", "unknown command type")
	}
}

func (s *State) ViewForSeat(seat int) MatchView {
	players := make([]PlayerView, 0, 2)
	for playerSeat := 0; playerSeat < 2; playerSeat++ {
		player := s.Players[playerSeat]
		view := PlayerView{
			Seat:       player.Seat,
			HeroID:     player.HeroID,
			HP:         player.HP,
			Energy:     player.Energy,
			TurnNumber: player.TurnNumber,
			DeckCount:  len(player.Deck),
			HandCount:  len(player.Hand),
			Hand:       []CardView{},
			Board:      make([]UnitView, 0, len(player.Board)),
		}
		if playerSeat == seat {
			for _, card := range player.Hand {
				view.Hand = append(view.Hand, s.cardView(card))
			}
		}
		for _, unit := range player.Board {
			definition := s.catalog[unit.CardID]
			view.Board = append(view.Board, UnitView{
				InstanceID:       unit.InstanceID,
				CardID:           unit.CardID,
				Name:             definition.Name,
				Attack:           unit.Attack,
				Health:           unit.Health,
				MaxHealth:        unit.MaxHealth,
				Guard:            definition.Guard,
				AttacksRemaining: unit.AttacksRemaining,
			})
		}
		players = append(players, view)
	}

	return MatchView{
		MatchID:      s.MatchID,
		Revision:     s.Revision,
		LastEventSeq: s.LastEventSeq,
		Turn:         s.Turn,
		ActiveSeat:   s.ActiveSeat,
		Status:       s.Status,
		WinnerSeat:   s.WinnerSeat,
		YouSeat:      seat,
		Players:      players,
	}
}

func EventsForSeat(events []Event, seat int) []Event {
	filtered := make([]Event, 0, len(events))
	for _, event := range events {
		if event.Visibility == VisibilityPublic || event.Visibility == SeatVisibility(seat) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func SeatVisibility(seat int) Visibility {
	if seat == 0 {
		return VisibilitySeat0
	}
	return VisibilitySeat1
}

func (s *State) playCard(seat int, command PlayerCommand) CommandResult {
	card, handIndex, ok := s.findHandCard(seat, command.CardInstanceID)
	if !ok {
		return s.reject("invalid_card", "card is not in your hand")
	}
	definition, ok := s.catalog[card.CardID]
	if !ok {
		return s.reject("invalid_card", "card definition is missing")
	}
	if s.Players[seat].Energy < definition.Cost {
		return s.reject("insufficient_energy", fmt.Sprintf("card costs %d energy, %d available", definition.Cost, s.Players[seat].Energy))
	}
	if definition.Kind == CardKindUnit && len(s.Players[seat].Board) >= BoardLimit {
		return s.reject("board_full", "board is full")
	}
	if err := s.validateSpellTarget(seat, definition, command.TargetID); err != nil {
		return s.reject(err.Code, err.Message)
	}

	cardValue := *card
	s.Revision++
	player := &s.Players[seat]
	player.Energy -= definition.Cost
	player.Hand = append(player.Hand[:handIndex], player.Hand[handIndex+1:]...)
	s.emit(seat, VisibilityPublic, "card_played", map[string]any{
		"cardInstanceId": cardValue.InstanceID,
		"cardId":         cardValue.CardID,
	})

	switch definition.Kind {
	case CardKindUnit:
		unit := CardInstance{
			InstanceID:       cardValue.InstanceID,
			CardID:           cardValue.CardID,
			Attack:           definition.Attack,
			Health:           definition.Health,
			MaxHealth:        definition.Health,
			AttacksRemaining: 0,
		}
		player.Board = append(player.Board, unit)
		s.emit(seat, VisibilityPublic, "unit_summoned", map[string]any{
			"instanceId": unit.InstanceID,
			"cardId":     unit.CardID,
			"attack":     unit.Attack,
			"health":     unit.Health,
			"guard":      definition.Guard,
		})
	case CardKindSpell:
		s.resolveSpell(seat, definition, command.TargetID)
	}

	s.checkWin()
	return s.accepted()
}

func (s *State) attack(seat int, command PlayerCommand) CommandResult {
	attackerIndex := -1
	for index := range s.Players[seat].Board {
		if s.Players[seat].Board[index].InstanceID == command.CardInstanceID {
			attackerIndex = index
			break
		}
	}
	if attackerIndex < 0 {
		return s.reject("invalid_card", "attacker is not on your board")
	}
	attacker := &s.Players[seat].Board[attackerIndex]
	if attacker.AttacksRemaining <= 0 {
		return s.reject("invalid_target", "unit cannot attack this turn")
	}

	opponentSeat := 1 - seat
	defenderIndex := -1
	for index := range s.Players[opponentSeat].Board {
		if s.Players[opponentSeat].Board[index].InstanceID == command.TargetID {
			defenderIndex = index
			break
		}
	}
	attacksHero := command.TargetID == HeroID(opponentSeat)
	if !attacksHero && defenderIndex < 0 {
		return s.reject("invalid_target", "target is not a valid enemy")
	}
	if defenderIndex >= 0 {
		hasGuard := false
		for _, unit := range s.Players[opponentSeat].Board {
			if s.catalog[unit.CardID].Guard {
				hasGuard = true
				break
			}
		}
		defenderDefinition := s.catalog[s.Players[opponentSeat].Board[defenderIndex].CardID]
		if hasGuard && !defenderDefinition.Guard {
			return s.reject("invalid_target", "a guard unit must be attacked first")
		}
	}

	s.Revision++
	attacker = &s.Players[seat].Board[attackerIndex]
	attacker.AttacksRemaining--
	if attacksHero {
		damage := attacker.Attack
		s.Players[opponentSeat].HP -= damage
		s.emit(seat, VisibilityPublic, "attack_resolved", map[string]any{
			"attackerId": attacker.InstanceID,
			"targetId":   HeroID(opponentSeat),
			"damage":     damage,
		})
		s.emit(seat, VisibilityPublic, "damage_dealt", map[string]any{
			"targetId": HeroID(opponentSeat),
			"amount":   damage,
			"hpAfter":  s.Players[opponentSeat].HP,
		})
	} else {
		defender := &s.Players[opponentSeat].Board[defenderIndex]
		attackerDamage := attacker.Attack
		defenderDamage := defender.Attack
		attacker.Health -= defenderDamage
		defender.Health -= attackerDamage
		s.emit(seat, VisibilityPublic, "attack_resolved", map[string]any{
			"attackerId":     attacker.InstanceID,
			"targetId":       defender.InstanceID,
			"attackerDamage": attackerDamage,
			"defenderDamage": defenderDamage,
		})
		s.emit(seat, VisibilityPublic, "damage_dealt", map[string]any{
			"targetId": defender.InstanceID,
			"amount":   attackerDamage,
			"hpAfter":  defender.Health,
		})
		s.emit(seat, VisibilityPublic, "damage_dealt", map[string]any{
			"targetId": attacker.InstanceID,
			"amount":   defenderDamage,
			"hpAfter":  attacker.Health,
		})
		s.removeDead(seat, seat)
		s.removeDead(opponentSeat, seat)
	}

	s.checkWin()
	return s.accepted()
}

// endTurn 结束当前回合：Action -> Ended，然后由对手开始新回合（Ended -> Start -> Draw -> Action）。
// 对局在本回合内已经结束时不再开始新回合。
func (s *State) endTurn(seat int) CommandResult {
	if seat != s.ActiveSeat {
		return s.reject("not_your_turn", "it is not your turn")
	}
	// 修改任何状态之前先校验本次指令所需的全部迁移，保证失败时不留下半个回合。
	if !canAdvance(s.Phase, PhaseEnded) || !canBeginTurnFrom(PhaseEnded) {
		return s.reject("invalid_command", fmt.Sprintf("回合阶段 %s 不允许结束回合", s.Phase))
	}
	if err := s.advancePhase(PhaseEnded); err != nil {
		return s.reject("invalid_command", err.Error())
	}
	s.Revision++
	s.emit(seat, VisibilityPublic, "turn_ended", map[string]any{
		"seat": seat,
	})
	s.checkWin()
	if s.Status == StatusFinished {
		return s.accepted()
	}

	if err := s.beginTurn(1-seat, true); err != nil {
		return s.reject("invalid_command", err.Error())
	}
	return s.accepted()
}

// surrender 立即结束对局，对手获胜。认输不要求当前是自己的回合。
func (s *State) surrender(seat int) CommandResult {
	if seat < 0 || seat > 1 {
		return s.reject("invalid_command", "invalid seat")
	}
	s.Revision++
	s.finish(1-seat, EndReasonSurrender, seat)
	return s.accepted()
}

func (s *State) draw(seat, count int, emit bool) {
	player := &s.Players[seat]
	for range count {
		if len(player.Deck) == 0 {
			player.Fatigue++
			player.HP -= player.Fatigue
			if emit {
				s.emit(seat, VisibilityPublic, "fatigue_damage", map[string]any{
					"seat":    seat,
					"amount":  player.Fatigue,
					"hpAfter": player.HP,
				})
			}
			continue
		}

		card := player.Deck[0]
		player.Deck = player.Deck[1:]
		if len(player.Hand) >= HandLimit {
			if emit {
				s.emit(seat, SeatVisibility(seat), "card_burned", map[string]any{
					"cardId": card.CardID,
				})
			}
			continue
		}
		player.Hand = append(player.Hand, card)
		if emit {
			s.emit(seat, SeatVisibility(seat), "card_drawn", map[string]any{
				"cardInstanceId": card.InstanceID,
				"cardId":         card.CardID,
			})
		}
	}
}

func (s *State) resolveSpell(seat int, definition CardDefinition, targetID string) {
	switch definition.ID {
	case "arcane_bolt":
		s.damageTarget(seat, targetID, 3)
	case "healing_light":
		s.healTarget(seat, targetID, 5)
	}
}

func (s *State) damageTarget(actorSeat int, targetID string, amount int) {
	for seat := 0; seat < 2; seat++ {
		if targetID == HeroID(seat) {
			s.Players[seat].HP -= amount
			s.emit(actorSeat, VisibilityPublic, "damage_dealt", map[string]any{
				"targetId": targetID,
				"amount":   amount,
				"hpAfter":  s.Players[seat].HP,
			})
			return
		}
		for index := range s.Players[seat].Board {
			unit := &s.Players[seat].Board[index]
			if unit.InstanceID != targetID {
				continue
			}
			unit.Health -= amount
			s.emit(actorSeat, VisibilityPublic, "damage_dealt", map[string]any{
				"targetId": unit.InstanceID,
				"amount":   amount,
				"hpAfter":  unit.Health,
			})
			s.removeDead(seat, actorSeat)
			return
		}
	}
}

func (s *State) healTarget(actorSeat int, targetID string, amount int) {
	for seat := 0; seat < 2; seat++ {
		if targetID == HeroID(seat) {
			before := s.Players[seat].HP
			s.Players[seat].HP = min(StartingHeroHP, s.Players[seat].HP+amount)
			s.emit(actorSeat, VisibilityPublic, "healed", map[string]any{
				"targetId": targetID,
				"amount":   s.Players[seat].HP - before,
				"hpAfter":  s.Players[seat].HP,
			})
			return
		}
		for index := range s.Players[seat].Board {
			unit := &s.Players[seat].Board[index]
			if unit.InstanceID != targetID {
				continue
			}
			before := unit.Health
			unit.Health = min(unit.MaxHealth, unit.Health+amount)
			s.emit(actorSeat, VisibilityPublic, "healed", map[string]any{
				"targetId": unit.InstanceID,
				"amount":   unit.Health - before,
				"hpAfter":  unit.Health,
			})
			return
		}
	}
}

func (s *State) validateSpellTarget(seat int, definition CardDefinition, targetID string) *CommandError {
	if definition.Kind != CardKindSpell {
		if targetID != "" {
			return &CommandError{Code: "invalid_target", Message: "unit cards do not take a target"}
		}
		return nil
	}

	switch definition.ID {
	case "arcane_bolt":
		if targetID == HeroID(1-seat) {
			return nil
		}
		if s.findUnit(targetID) >= 0 {
			return nil
		}
		return &CommandError{Code: "invalid_target", Message: "arcane_bolt requires an enemy hero or any unit"}
	case "healing_light":
		if targetID == HeroID(seat) || s.findFriendlyUnit(seat, targetID) >= 0 {
			return nil
		}
		return &CommandError{Code: "invalid_target", Message: "healing_light requires a friendly hero or unit"}
	default:
		return &CommandError{Code: "invalid_card", Message: "spell has no target rule"}
	}
}

func (s *State) findHandCard(seat int, instanceID string) (*CardInstance, int, bool) {
	for index := range s.Players[seat].Hand {
		card := &s.Players[seat].Hand[index]
		if card.InstanceID == instanceID {
			return card, index, true
		}
	}
	return nil, -1, false
}

func (s *State) findUnit(instanceID string) int {
	for seat := 0; seat < 2; seat++ {
		index := s.findFriendlyUnit(seat, instanceID)
		if index >= 0 {
			return index
		}
	}
	return -1
}

func (s *State) findFriendlyUnit(seat int, instanceID string) int {
	for index := range s.Players[seat].Board {
		if s.Players[seat].Board[index].InstanceID == instanceID {
			return index
		}
	}
	return -1
}

func (s *State) removeDead(seat int, actorSeat int) {
	player := &s.Players[seat]
	alive := player.Board[:0]
	for _, unit := range player.Board {
		if unit.Health > 0 {
			alive = append(alive, unit)
			continue
		}
		s.emit(actorSeat, VisibilityPublic, "unit_died", map[string]any{
			"instanceId": unit.InstanceID,
			"cardId":     unit.CardID,
		})
	}
	player.Board = alive
}

func (s *State) cardView(card CardInstance) CardView {
	definition := s.catalog[card.CardID]
	return CardView{
		InstanceID: card.InstanceID,
		CardID:     card.CardID,
		Name:       definition.Name,
		Cost:       definition.Cost,
		Kind:       definition.Kind,
	}
}

func (s *State) accepted() CommandResult {
	return CommandResult{
		Accepted: true,
		Revision: s.Revision,
		Events:   s.drainEvents(),
	}
}

func (s *State) reject(code, message string) CommandResult {
	s.pendingEvents = nil
	return CommandResult{
		Accepted: false,
		Revision: s.Revision,
		Events:   []Event{},
		Error:    &CommandError{Code: code, Message: message},
	}
}

func (s *State) emit(actorSeat int, visibility Visibility, eventType string, data map[string]any) {
	s.LastEventSeq++
	s.pendingEvents = append(s.pendingEvents, Event{
		Seq:        s.LastEventSeq,
		Revision:   s.Revision,
		Type:       eventType,
		ActorSeat:  actorSeat,
		Visibility: visibility,
		Data:       data,
	})
}

func (s *State) drainEvents() []Event {
	events := s.pendingEvents
	s.pendingEvents = nil
	return events
}
