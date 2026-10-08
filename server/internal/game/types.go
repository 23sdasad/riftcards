package game

import "math/rand"

type CommandType string

const (
	CommandPlayCard CommandType = "play_card"
	CommandAttack   CommandType = "attack"
	CommandEndTurn  CommandType = "end_turn"
	CommandSurrender CommandType = "surrender"
)

type CardKind string

const (
	CardKindUnit  CardKind = "unit"
	CardKindSpell CardKind = "spell"
)

type MatchStatus string

const (
	StatusActive   MatchStatus = "active"
	StatusFinished MatchStatus = "finished"
)

type Visibility string

const (
	VisibilityPublic Visibility = "public"
	VisibilitySeat0  Visibility = "seat:0"
	VisibilitySeat1  Visibility = "seat:1"
)

type CardDefinition struct {
	ID     string
	Name   string
	Cost   int
	Kind   CardKind
	Attack int
	Health int
	Guard  bool
	Text   string
}

type CardInstance struct {
	InstanceID       string
	CardID           string
	Attack           int
	Health           int
	MaxHealth        int
	AttacksRemaining int
}

type PlayerState struct {
	Seat       int
	HeroID     string
	HP         int
	Energy     int
	TurnNumber int
	Fatigue    int
	Deck       []CardInstance
	Hand       []CardInstance
	Board      []CardInstance
}

type State struct {
	MatchID       string
	Revision      int64
	LastEventSeq  int64
	Turn          int
	ActiveSeat    int
	Status        MatchStatus
	WinnerSeat    *int
	Players       [2]PlayerState
	firstSeat     int
	catalog       map[string]CardDefinition
	rng           *rand.Rand
	pendingEvents []Event
}

type PlayerCommand struct {
	Type           CommandType `json:"type"`
	CardInstanceID string      `json:"cardInstanceId,omitempty"`
	TargetID       string      `json:"targetId,omitempty"`
}

type CommandError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CommandResult struct {
	Accepted bool
	Revision int64
	Events   []Event
	Error    *CommandError
}

type Event struct {
	Seq        int64          `json:"seq"`
	Revision   int64          `json:"revision"`
	Type       string         `json:"type"`
	ActorSeat  int            `json:"actorSeat"`
	Visibility Visibility     `json:"visibility"`
	Data       map[string]any `json:"data"`
}

type MatchView struct {
	MatchID      string       `json:"matchId"`
	Revision     int64        `json:"revision"`
	LastEventSeq int64        `json:"lastEventSeq"`
	Turn         int          `json:"turn"`
	ActiveSeat   int          `json:"activeSeat"`
	Status       MatchStatus  `json:"status"`
	WinnerSeat   *int         `json:"winnerSeat"`
	YouSeat      int          `json:"youSeat"`
	Players      []PlayerView `json:"players"`
}

type PlayerView struct {
	Seat       int        `json:"seat"`
	HeroID     string     `json:"heroId"`
	HP         int        `json:"hp"`
	Energy     int        `json:"energy"`
	TurnNumber int        `json:"turnNumber"`
	DeckCount  int        `json:"deckCount"`
	HandCount  int        `json:"handCount"`
	Hand       []CardView `json:"hand"`
	Board      []UnitView `json:"board"`
}

type CardView struct {
	InstanceID string   `json:"instanceId"`
	CardID     string   `json:"cardId"`
	Name       string   `json:"name"`
	Cost       int      `json:"cost"`
	Kind       CardKind `json:"kind"`
}

type UnitView struct {
	InstanceID       string `json:"instanceId"`
	CardID           string `json:"cardId"`
	Name             string `json:"name"`
	Attack           int    `json:"attack"`
	Health           int    `json:"health"`
	MaxHealth        int    `json:"maxHealth"`
	Guard            bool   `json:"guard"`
	AttacksRemaining int    `json:"attacksRemaining"`
}
