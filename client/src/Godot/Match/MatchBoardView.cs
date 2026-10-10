using Godot;
using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;

namespace Riftcards.Client.Godot.Match;

/// <summary>
/// 对局面板：把 <see cref="MatchBoard"/> 的绑定状态渲染成可操作界面，并把点击转换成意图。
///
/// 边界：面板不计算费用、伤害、目标合法性或胜负，只显示服务端投影并提交玩家意图；
/// 非法操作由服务端拒绝，错误码显示在提示行。
/// </summary>
public sealed partial class MatchBoardView : VBoxContainer
{
    private readonly MatchBoard _board = new();
    private readonly List<CardButton> _handButtons = [];
    private readonly List<UnitButton> _ownUnitButtons = [];
    private readonly List<UnitButton> _opponentUnitButtons = [];

    private GameSession? _session;
    private Label _turnLabel = null!;
    private Label _hintLabel = null!;
    private Label _handLabel = null!;
    private Label _boardLabel = null!;
    private HBoxContainer _handRow = null!;
    private HBoxContainer _ownBoardRow = null!;
    private HBoxContainer _opponentBoardRow = null!;
    private HeroPanel _ownHero = null!;
    private HeroPanel _opponentHero = null!;
    private Button _endTurnButton = null!;
    private Button _surrenderButton = null!;

    public override void _Ready()
    {
        AddThemeConstantOverride("separation", 6);
        BuildUi();
        Refresh();
    }

    /// <summary>绑定会话；重复调用会替换旧会话的事件订阅。</summary>
    public void Bind(GameSession session)
    {
        if (_session is not null)
        {
            _session.Changed -= Refresh;
        }
        _session = session;
        _session.Changed += Refresh;
        Refresh();
    }

    /// <summary>按当前投影刷新界面。</summary>
    public void Refresh()
    {
        _board.Apply(_session?.View);
        _hintLabel.Text = HintText();

        if (!_board.HasMatch)
        {
            _turnLabel.Text = "未进入对局";
            _handLabel.Text = "手牌";
            _boardLabel.Text = "场地";
            ClearButtons(_handButtons, _handRow);
            ClearButtons(_ownUnitButtons, _ownBoardRow);
            ClearButtons(_opponentUnitButtons, _opponentBoardRow);
            _endTurnButton.Disabled = true;
            _surrenderButton.Disabled = true;
            return;
        }

        var view = _board.View!;
        var you = _board.You;
        var opponent = _board.Opponent;
        if (you is null || opponent is null)
        {
            return;
        }

        _turnLabel.Text = _board.IsFinished
            ? $"对局结束（{view.Status}）"
            : _board.IsYourTurn
                ? $"你的回合（第 {view.Turn} 回合，revision {view.Revision}）"
                : $"对手回合（第 {view.Turn} 回合，revision {view.Revision}）";

        _ownHero.Configure(you, isYou: true, awaitingTarget: false, finished: _board.IsFinished);
        _opponentHero.Configure(opponent, isYou: false, awaitingTarget: _board.AwaitingTarget, finished: _board.IsFinished);

        _handLabel.Text = $"手牌（费用 {you.Energy}）";
        _boardLabel.Text = "场地";
        RebuildHand(you);
        RebuildBoard(_ownBoardRow, _ownUnitButtons, you, opponent: false);
        RebuildBoard(_opponentBoardRow, _opponentUnitButtons, opponent, opponent: true);

        _endTurnButton.Disabled = !_board.IsYourTurn;
        _surrenderButton.Disabled = _board.IsFinished;
    }

    private void BuildUi()
    {
        _turnLabel = new Label { AutowrapMode = TextServer.AutowrapMode.WordSmart };
        AddChild(_turnLabel);

        _opponentHero = new HeroPanel();
        AddChild(_opponentHero);
        _opponentHero.TargetPressed += OnHeroPressed;

        _opponentBoardRow = NewRow();
        AddChild(_opponentBoardRow);

        _ownBoardRow = NewRow();
        AddChild(_ownBoardRow);

        _ownHero = new HeroPanel();
        AddChild(_ownHero);

        _handLabel = new Label();
        AddChild(_handLabel);

        _handRow = NewRow();
        AddChild(_handRow);

        var actionRow = NewRow();
        AddChild(actionRow);

        _endTurnButton = new Button { Text = "结束回合" };
        _endTurnButton.Pressed += () => Submit(_board.EndTurn());
        actionRow.AddChild(_endTurnButton);

        _surrenderButton = new Button { Text = "认输" };
        _surrenderButton.Pressed += () => Submit(_board.Surrender());
        actionRow.AddChild(_surrenderButton);

        _boardLabel = new Label();
        AddChild(_boardLabel);

        _hintLabel = new Label { AutowrapMode = TextServer.AutowrapMode.WordSmart };
        AddChild(_hintLabel);
    }

    private static HBoxContainer NewRow()
    {
        var row = new HBoxContainer();
        row.AddThemeConstantOverride("separation", 6);
        return row;
    }

    private void RebuildHand(PlayerView you)
    {
        ClearButtons(_handButtons, _handRow);
        foreach (var card in you.Hand)
        {
            var button = new CardButton();
            _handRow.AddChild(button);
            button.Configure(card, card.InstanceId == _board.SelectedCardInstanceId, interactive: _board.IsYourTurn);
            var instanceId = card.InstanceId;
            button.Pressed += () => OnHandCardPressed(instanceId);
            _handButtons.Add(button);
        }
    }

    private void RebuildBoard(HBoxContainer row, List<UnitButton> buttons, PlayerView player, bool opponent)
    {
        ClearButtons(buttons, row);
        foreach (var unit in player.Board)
        {
            var button = new UnitButton();
            row.AddChild(button);
            var selected = !opponent && unit.InstanceId == _board.SelectedAttackerInstanceId;
            button.Configure(unit, opponent, selected, interactive: _board.IsYourTurn);
            var instanceId = unit.InstanceId;
            button.Pressed += () => OnUnitPressed(instanceId);
            buttons.Add(button);
        }
    }

    private void OnHandCardPressed(string instanceId) => Submit(_board.SelectHandCard(instanceId));

    private void OnUnitPressed(string instanceId)
    {
        // 等待法术目标时，点任意单位都是选目标；否则按己方/对手决定攻击者或攻击目标。
        var intent = _board.AwaitingTarget ? _board.SelectTarget(instanceId) : _board.SelectBoardUnit(instanceId);
        Submit(intent);
    }

    private void OnHeroPressed(int seat)
    {
        var intent = _board.AwaitingTarget ? _board.SelectTarget(HeroIdOf(seat) ?? string.Empty) : _board.SelectHero(seat);
        Submit(intent);
    }

    private void Submit(CommandIntent? intent)
    {
        if (intent is not null)
        {
            _session?.SubmitCommand(intent.Type, intent.CardInstanceId, intent.TargetId);
        }
        Refresh();
    }

    private string? HeroIdOf(int seat) =>
        _board.View?.Players.FirstOrDefault(player => player.Seat == seat)?.HeroId;

    /// <summary>提示行：优先显示服务端错误码，其次是当前需要玩家做的事。</summary>
    private string HintText()
    {
        if (_session is null)
        {
            return string.Empty;
        }
        if (_session.LastCommandOutcome is { Accepted: false } outcome)
        {
            return $"服务端拒绝：{outcome.ErrorCode ?? "unknown"}";
        }
        if (_session.LastError is { } error)
        {
            return $"服务端错误：{error.Code}（{error.Message}）";
        }
        if (!_board.HasMatch)
        {
            return "连接后点击 Queue 进入匹配。";
        }
        if (_board.AwaitingTarget)
        {
            return "请选择目标：点击任意单位或对手英雄。";
        }
        if (_board.Selection == SelectionKind.Attacker)
        {
            return "请选择攻击目标：点击对手单位或对手英雄。";
        }
        return _board.IsYourTurn ? "点击手牌出牌，或点己方单位后选择攻击目标。" : "等待对手行动。";
    }

    private static void ClearButtons<TButton>(List<TButton> buttons, Node container)
        where TButton : Node
    {
        foreach (var button in buttons)
        {
            container.RemoveChild(button);
            button.QueueFree();
        }
        buttons.Clear();
    }
}
