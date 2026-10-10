using Riftcards.Client.Core.Protocol;

namespace Riftcards.Client.Core.Session;

/// <summary>玩家意图：由界面收集后交给 <see cref="GameSession"/> 发送。</summary>
public sealed record CommandIntent(string Type, string? CardInstanceId = null, string? TargetId = null);

/// <summary>当前交互选择。</summary>
public enum SelectionKind
{
    /// <summary>没有选择。</summary>
    None,

    /// <summary>已选中一张手牌，正在等待目标（仅法术）。</summary>
    Card,

    /// <summary>已选中一个己方单位作为攻击者，正在等待攻击目标。</summary>
    Attacker,
}

/// <summary>
/// 对局界面的视图模型：把服务端投影转换成可绑定状态，并收集玩家意图。
///
/// 边界（见 ai-docs/product/game-rules.md 与 task 008）：
///   - 只读取投影里已有的字段，不读取对手手牌内容（对手只暴露数量）。
///   - 不判断费用、目标合法性、伤害或胜负：这些一律由服务端裁决。
///   - 只根据投影做交互可用性（是否轮到自己、对局是否结束）和选择状态管理。
/// </summary>
public sealed class MatchBoard
{
    private MatchView? _view;
    private string? _selectedCardInstanceId;
    private string? _selectedAttackerInstanceId;

    /// <summary>用最新投影校正视图模型；无效选择会被清理。</summary>
    public void Apply(MatchView? view)
    {
        _view = view;
        PruneSelection();
    }

    public MatchView? View => _view;

    public bool HasMatch => _view is not null;

    /// <summary>对局是否已结束（状态不是 active）。</summary>
    public bool IsFinished => _view is not null && _view.Status != "active";

    /// <summary>是否轮到自己行动。</summary>
    public bool IsYourTurn => _view is not null && !IsFinished && _view.ActiveSeat == _view.YouSeat;

    /// <summary>自己的投影；无对局时为 null。</summary>
    public PlayerView? You => _view is null ? null : FindPlayer(_view.YouSeat);

    /// <summary>对手的投影；对手手牌内容始终为空，只有数量。</summary>
    public PlayerView? Opponent => _view is null ? null : _view.Players.FirstOrDefault(player => player.Seat != _view.YouSeat);

    public SelectionKind Selection { get; private set; }

    public string? SelectedCardInstanceId => _selectedCardInstanceId;

    public string? SelectedAttackerInstanceId => _selectedAttackerInstanceId;

    /// <summary>是否正在等待玩家点选目标。</summary>
    public bool AwaitingTarget => Selection == SelectionKind.Card;

    /// <summary>点击手牌：单位立即产生出牌意图，法术进入等待目标状态。</summary>
    public CommandIntent? SelectHandCard(string instanceId)
    {
        if (!IsYourTurn)
        {
            return null;
        }

        var card = You?.Hand.FirstOrDefault(candidate => candidate.InstanceId == instanceId);
        if (card is null)
        {
            return null;
        }

        _selectedAttackerInstanceId = null;
        _selectedCardInstanceId = instanceId;
        Selection = SelectionKind.Card;

        if (card.Kind == ProtocolNames.CardKindSpell)
        {
            // 目标是否合法由服务端判断，这里只等待玩家选择。
            return null;
        }

        ClearSelection();
        return new CommandIntent(ProtocolNames.PlayCard, instanceId);
    }

    /// <summary>点击目标（任意可见单位或英雄）。不判断目标是否合法。</summary>
    public CommandIntent? SelectTarget(string targetId)
    {
        if (!IsYourTurn || Selection != SelectionKind.Card || _selectedCardInstanceId is null)
        {
            return null;
        }

        var cardInstanceId = _selectedCardInstanceId;
        ClearSelection();
        return new CommandIntent(ProtocolNames.PlayCard, cardInstanceId, targetId);
    }

    /// <summary>点击场地单位：己方单位成为攻击者，对手单位作为攻击目标。</summary>
    public CommandIntent? SelectBoardUnit(string instanceId)
    {
        if (!IsYourTurn)
        {
            return null;
        }

        if (You?.Board.Any(unit => unit.InstanceId == instanceId) == true)
        {
            _selectedCardInstanceId = null;
            _selectedAttackerInstanceId = instanceId;
            Selection = SelectionKind.Attacker;
            return null;
        }

        if (Selection != SelectionKind.Attacker || _selectedAttackerInstanceId is null)
        {
            return null;
        }
        if (Opponent?.Board.Any(unit => unit.InstanceId == instanceId) != true)
        {
            return null;
        }

        var attacker = _selectedAttackerInstanceId;
        ClearSelection();
        return new CommandIntent(ProtocolNames.Attack, attacker, instanceId);
    }

    /// <summary>点击英雄：已选攻击者时，点击对手英雄产生攻击意图。</summary>
    public CommandIntent? SelectHero(int seat)
    {
        if (!IsYourTurn || Selection != SelectionKind.Attacker || _selectedAttackerInstanceId is null)
        {
            return null;
        }

        var opponent = Opponent;
        if (opponent is null || seat != opponent.Seat)
        {
            // 规则不允许攻击友方英雄；这不是客户端判断，而是协议里不存在该意图。
            return null;
        }

        var attacker = _selectedAttackerInstanceId;
        ClearSelection();
        return new CommandIntent(ProtocolNames.Attack, attacker, opponent.HeroId);
    }

    /// <summary>结束回合意图；不在自己的回合时返回 null。</summary>
    public CommandIntent? EndTurn() => IsYourTurn ? new CommandIntent(ProtocolNames.EndTurn) : null;

    /// <summary>认输意图；对局进行中随时可用（含对手回合）。</summary>
    public CommandIntent? Surrender() => HasMatch && !IsFinished ? new CommandIntent(ProtocolNames.Surrender) : null;

    /// <summary>清空当前选择。</summary>
    public void ClearSelection()
    {
        _selectedCardInstanceId = null;
        _selectedAttackerInstanceId = null;
        Selection = SelectionKind.None;
    }

    private PlayerView? FindPlayer(int seat) =>
        _view?.Players.FirstOrDefault(player => player.Seat == seat);

    /// <summary>投影变化后清理已经无效的选择，避免界面停留在过期状态。</summary>
    private void PruneSelection()
    {
        if (_view is null || IsFinished || !IsYourTurn)
        {
            ClearSelection();
            return;
        }

        if (_selectedCardInstanceId is not null &&
            You?.Hand.Any(card => card.InstanceId == _selectedCardInstanceId) != true)
        {
            ClearSelection();
            return;
        }

        if (_selectedAttackerInstanceId is not null &&
            You?.Board.Any(unit => unit.InstanceId == _selectedAttackerInstanceId) != true)
        {
            ClearSelection();
        }
    }
}
