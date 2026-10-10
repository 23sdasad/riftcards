package game

import "fmt"

// 本文件是对局与回合生命周期状态机的唯一定义位置。
//
// 两个维度互相独立：
//   - 生命周期：Status（active -> finished），协议可见，且不可逆；
//   - 回合阶段：Phase（idle -> start -> draw -> action -> ended -> start ...），引擎内部。
//
// 回合阶段不对客户端暴露：客户端用 status 与 activeSeat 判断自己能否行动，
// 因此状态机第一版不改变任何协议字段（见 ai-docs/architecture/match-state-machine.md）。

// turnTransitions 定义回合阶段的合法迁移。不在此表中的迁移一律拒绝，
// 且拒绝时不得修改任何权威状态（包括 Phase、Revision 与 LastEventSeq）。
var turnTransitions = map[TurnPhase][]TurnPhase{
	PhaseIdle:   {PhaseStart},
	PhaseStart:  {PhaseDraw},
	PhaseDraw:   {PhaseAction},
	PhaseAction: {PhaseEnded},
	PhaseEnded:  {PhaseStart},
}

// turnStartPath 是一个回合从开始到可行动所经过的固定阶段顺序。
var turnStartPath = []TurnPhase{PhaseStart, PhaseDraw, PhaseAction}

// canAdvance 判断能否从 from 迁移到 next。
func canAdvance(from, next TurnPhase) bool {
	for _, allowed := range turnTransitions[from] {
		if allowed == next {
			return true
		}
	}
	return false
}

// canBeginTurnFrom 判断从 from 阶段能否走完整个回合开始路径。
// 用于在修改状态之前一次性校验，避免出现“改了一半”的回合。
func canBeginTurnFrom(from TurnPhase) bool {
	for _, next := range turnStartPath {
		if !canAdvance(from, next) {
			return false
		}
		from = next
	}
	return true
}

// advancePhase 校验并执行一次阶段迁移。非法迁移返回错误，且不修改状态。
func (s *State) advancePhase(next TurnPhase) error {
	if !canAdvance(s.Phase, next) {
		return fmt.Errorf("非法回合阶段迁移：%s -> %s", s.Phase, next)
	}
	s.Phase = next
	return nil
}

// beginTurn 按固定顺序推进一个回合：Start（费用与攻击权）-> Draw（抽牌）-> Action。
//
// 规则 3.4：先手玩家的第一回合不抽牌，此时 Draw 阶段不产生任何事件。
// emit 为 false 时只推进状态、不产生事件，用于开局。
func (s *State) beginTurn(seat int, emit bool) error {
	if !canBeginTurnFrom(s.Phase) {
		return fmt.Errorf("无法从阶段 %s 开始新回合", s.Phase)
	}
	// 上面的校验已排除失败可能；这里仍显式处理返回值，避免掩盖状态机缺陷。
	if err := s.advancePhase(PhaseStart); err != nil {
		return err
	}
	s.ActiveSeat = seat
	s.Turn++
	player := &s.Players[seat]
	player.TurnNumber++
	player.Energy = min(MaxEnergy, player.TurnNumber)
	for index := range player.Board {
		player.Board[index].AttacksRemaining = 1
	}
	if emit {
		s.emit(seat, VisibilityPublic, "turn_started", map[string]any{
			"seat":       seat,
			"turn":       s.Turn,
			"turnNumber": player.TurnNumber,
			"energy":     player.Energy,
		})
	}

	if err := s.advancePhase(PhaseDraw); err != nil {
		return err
	}
	// 本局第一个回合属于先手玩家，按规则不抽牌。
	if s.Turn != 1 || seat != s.firstSeat {
		s.draw(seat, 1, emit)
	}
	return s.advancePhase(PhaseAction)
}

// finish 执行唯一的终局迁移：写入胜者与结束原因，并产生 match_ended 事件。
// 已经结束的对局不会被再次改写，因此重复调用是安全的。
func (s *State) finish(winnerSeat int, reason EndReason, actorSeat int) {
	if s.Status == StatusFinished {
		return
	}
	s.Status = StatusFinished
	s.EndReason = reason
	s.WinnerSeat = &winnerSeat
	s.emit(actorSeat, VisibilityPublic, "match_ended", map[string]any{
		"winnerSeat": winnerSeat,
		"reason":     string(reason),
	})
}

// checkWin 在每次可能改变英雄生命之后调用：任一方英雄倒下即结束对局。
// 双方同时倒下时当前回合玩家获胜（规则 8，MVP 不设计同时死亡效果）。
func (s *State) checkWin() {
	if s.Status != StatusActive {
		return
	}
	seat0Dead := s.Players[0].HP <= 0
	seat1Dead := s.Players[1].HP <= 0
	if !seat0Dead && !seat1Dead {
		return
	}

	winner := s.ActiveSeat
	if seat0Dead && !seat1Dead {
		winner = 1
	}
	if seat1Dead && !seat0Dead {
		winner = 0
	}
	s.finish(winner, EndReasonHeroDefeated, s.ActiveSeat)
}
