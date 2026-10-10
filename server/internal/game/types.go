package game

import "math/rand"

type CommandType string

const (
	CommandPlayCard  CommandType = "play_card"
	CommandAttack    CommandType = "attack"
	CommandEndTurn   CommandType = "end_turn"
	CommandSurrender CommandType = "surrender"
)

type CardKind string

const (
	CardKindUnit  CardKind = "unit"
	CardKindSpell CardKind = "spell"
)

// MatchStatus 是对局生命周期状态，也是协议可见字段（MatchView.status）。
// 迁移只有一条：active -> finished，且不可逆。
type MatchStatus string

const (
	StatusActive   MatchStatus = "active"
	StatusFinished MatchStatus = "finished"
)

// EndReason 是对局结束的原因，取值与 match_ended 事件的 reason 字段一致。
type EndReason string

const (
	// EndReasonHeroDefeated 表示一方英雄生命降至 0 或以下。
	EndReasonHeroDefeated EndReason = "hero_defeated"
	// EndReasonSurrender 表示一方认输（MVP 中断线也按此处理）。
	EndReasonSurrender EndReason = "surrender"
)

// TurnPhase 是回合内部阶段。它属于引擎内部状态：协议不暴露该字段，
// 客户端用 status 与 activeSeat 判断自己能否行动。
type TurnPhase string

const (
	// PhaseIdle 是对局尚未开始第一个回合时的阶段。
	PhaseIdle TurnPhase = "idle"
	// PhaseStart 正在执行回合开始：重置费用并恢复己方单位的攻击次数。
	PhaseStart TurnPhase = "start"
	// PhaseDraw 正在抽牌，包含爆牌与疲劳伤害。
	PhaseDraw TurnPhase = "draw"
	// PhaseAction 可以出牌、攻击、认输或结束回合。
	PhaseAction TurnPhase = "action"
	// PhaseEnded 本回合已结束，等待对手回合开始。
	PhaseEnded TurnPhase = "ended"
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
	MatchID      string
	Revision     int64
	LastEventSeq int64
	Turn         int
	ActiveSeat   int
	Status       MatchStatus
	// Phase 是当前回合阶段；对局结束时保持在当时的阶段，生命周期以 Status 为准。
	Phase TurnPhase
	// EndReason 只在 Status 为 StatusFinished 时有值。
	EndReason     EndReason
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

// 引擎产生的错误码。传输与会话级错误码见 protocol 包的 Code* 常量；
// 每条错误码的产生位置与响应载体见 ai-docs/contracts/protocol.md。
const (
	// CodeInvalidCommand 指令类型未知，或当前回合阶段不接受玩家指令。
	CodeInvalidCommand = "invalid_command"
	// CodeNotYourTurn 指令来自非当前回合玩家。
	CodeNotYourTurn = "not_your_turn"
	// CodeInvalidCard 卡牌实例不在手牌或场上，或卡牌定义缺失。
	CodeInvalidCard = "invalid_card"
	// CodeInsufficientEnergy 费用不足。
	CodeInsufficientEnergy = "insufficient_energy"
	// CodeInvalidTarget 目标不合法（含必须优先攻击守卫的情况）。
	CodeInvalidTarget = "invalid_target"
	// CodeBoardFull 场地已满，无法召唤单位。
	CodeBoardFull = "board_full"
	// CodeMatchFinished 对局已结束，拒绝所有会改变状态的指令。
	CodeMatchFinished = "match_finished"
)

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
