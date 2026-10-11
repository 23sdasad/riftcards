using Godot;
using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;

namespace Riftcards.Client.Godot.Match;

/// <summary>
/// 对局面板：把 <see cref="MatchBoard"/> 的绑定状态渲染成可操作界面，并把点击转换成意图。
///
/// 布局遵循 ai-docs/architecture/client-ui.md：对手在上、自己在下，回合条（含结束回合与认输）
/// 紧跟对手区之后，因此窗口变小时操作仍在首屏；场地与手牌用横向滚动，事件面板由应用壳承载。
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
    private PanelContainer _banner = null!;
    private Label _bannerLabel = null!;
    private Label _turnLabel = null!;
    private Label _hintLabel = null!;
    private Label _handTitle = null!;
    private HBoxContainer _handRow = null!;
    private HBoxContainer _ownBoardRow = null!;
    private HBoxContainer _opponentBoardRow = null!;
    private HeroPanel _ownHero = null!;
    private HeroPanel _opponentHero = null!;
    private Button _endTurnButton = null!;
    private Button _surrenderButton = null!;

    private bool _autoPlay;
    private bool _autoPlayEndedTurn;
    private bool _autoPlayReportedWait;
    private readonly HashSet<string> _autoPlayAttempted = [];

    /// <summary>
    /// 开发用自动出牌：走与点击完全相同的意图路径（<see cref="MatchBoard"/> + <see cref="Submit"/>），
    /// 让无人工操作的冒烟与截图也能看到真实对局状态。只由 `--smoke-autoplay` 开启。
    /// </summary>
    public void EnableAutoPlay() => _autoPlay = true;

    public override void _Process(double delta)
    {
        if (!_autoPlay || _session is null)
        {
            return;
        }
        if (_session.PendingRequests > 0)
        {
            if (!_autoPlayReportedWait)
            {
                _autoPlayReportedWait = true;
                GD.Print($"[riftcards] autoplay waiting: pending={_session.PendingRequests}");
            }
            return;
        }
        AutoPlayStep();
    }

    public override void _Ready()
    {
        AddThemeConstantOverride("separation", UiTheme.SectionSeparation);
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
        _hintLabel.AddThemeColorOverride("font_color", HintColor());

        var view = _board.View;
        var you = _board.You;
        var opponent = _board.Opponent;
        if (view is null || you is null || opponent is null)
        {
            _turnLabel.Text = "未进入对局";
            _turnLabel.AddThemeColorOverride("font_color", UiTheme.TextMuted);
            _handTitle.Text = "手牌";
            _banner.Visible = false;
            _opponentHero.ConfigureEmpty("对手");
            _ownHero.ConfigureEmpty("你");
            ClearButtons(_handButtons, _handRow);
            ClearButtons(_ownUnitButtons, _ownBoardRow);
            ClearButtons(_opponentUnitButtons, _opponentBoardRow);
            _endTurnButton.Disabled = true;
            _surrenderButton.Disabled = true;
            return;
        }

        var activeIsYou = view.ActiveSeat == view.YouSeat;
        _turnLabel.Text = $"第 {view.Turn} 回合 · {(activeIsYou ? "你的回合" : "对手回合")} · revision {view.Revision}";
        _turnLabel.AddThemeColorOverride("font_color", activeIsYou ? UiTheme.Accent : UiTheme.Warning);

        _opponentHero.Configure(opponent, isYou: false, active: !activeIsYou,
            awaitingTarget: _board.AwaitingTarget, finished: _board.IsFinished);
        _ownHero.Configure(you, isYou: true, active: activeIsYou,
            awaitingTarget: false, finished: _board.IsFinished);

        _handTitle.Text = $"手牌（费用 {you.Energy}）";
        RebuildHand(you);
        RebuildBoard(_ownBoardRow, _ownUnitButtons, you, opponent: false);
        RebuildBoard(_opponentBoardRow, _opponentUnitButtons, opponent, opponent: true);

        _endTurnButton.Disabled = !_board.IsYourTurn;
        _surrenderButton.Disabled = _board.IsFinished;
        UpdateBanner();
    }

    private void BuildUi()
    {
        // 终局横幅：默认隐藏。
        _bannerLabel = UiTheme.Heading(string.Empty);
        _banner = UiTheme.PanelBox(_bannerLabel, UiTheme.Accent, Color.FromHtml("20262f"));
        _banner.Visible = false;
        AddChild(_banner);

        // 中间是可滚动的战场：窗口变小时只压缩它，不压缩操作区。
        var scroll = new ScrollContainer
        {
            SizeFlagsHorizontal = SizeFlags.ExpandFill,
            SizeFlagsVertical = SizeFlags.ExpandFill,
            HorizontalScrollMode = ScrollContainer.ScrollMode.Disabled,
        };
        AddChild(scroll);

        var board = new VBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        board.AddThemeConstantOverride("separation", UiTheme.SectionSeparation);
        scroll.AddChild(board);

        // 对手区：英雄 + 场地。
        var opponentColumn = new VBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        opponentColumn.AddThemeConstantOverride("separation", UiTheme.ElementSeparation);
        _opponentHero = new HeroPanel();
        _opponentHero.TargetPressed += OnHeroPressed;
        opponentColumn.AddChild(_opponentHero);
        _opponentBoardRow = NewUnitRow(out var opponentScroll);
        opponentColumn.AddChild(LabelledRow("场地", opponentScroll));
        board.AddChild(UiTheme.PanelBox(opponentColumn));

        // 己方区：场地 + 英雄。
        var ownColumn = new VBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        ownColumn.AddThemeConstantOverride("separation", UiTheme.ElementSeparation);
        _ownBoardRow = NewUnitRow(out var ownScroll);
        ownColumn.AddChild(LabelledRow("场地", ownScroll));
        _ownHero = new HeroPanel();
        ownColumn.AddChild(_ownHero);
        board.AddChild(UiTheme.PanelBox(ownColumn));

        // 提示行紧随战场，说明当前该做什么或服务端返回的错误。
        _hintLabel = UiTheme.Body(string.Empty, UiTheme.TextSecondary);
        _hintLabel.AutowrapMode = TextServer.AutowrapMode.WordSmart;
        AddChild(_hintLabel);

        // 手牌固定在底部（卡牌游戏习惯），窗口再小也可见。
        var handColumn = new VBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        handColumn.AddThemeConstantOverride("separation", UiTheme.ElementSeparation);
        _handTitle = UiTheme.Heading("手牌");
        handColumn.AddChild(_handTitle);
        _handRow = NewUnitRow(out var handScroll);
        handColumn.AddChild(handScroll);
        AddChild(UiTheme.PanelBox(handColumn));

        // 回合条也固定在底部：结束回合与认输永远在首屏。
        var turnRow = new HBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        turnRow.AddThemeConstantOverride("separation", UiTheme.ElementSeparation);
        _turnLabel = UiTheme.Body("未进入对局", UiTheme.TextMuted);
        _turnLabel.SizeFlagsHorizontal = SizeFlags.ExpandFill;
        turnRow.AddChild(_turnLabel);

        _endTurnButton = new Button { Text = "结束回合" };
        UiTheme.StyleButton(_endTurnButton, UiTheme.Accent);
        _endTurnButton.Pressed += () => Submit(_board.EndTurn());
        turnRow.AddChild(_endTurnButton);

        _surrenderButton = new Button { Text = "认输" };
        UiTheme.StyleButton(_surrenderButton, UiTheme.Danger);
        _surrenderButton.Pressed += () => Submit(_board.Surrender());
        turnRow.AddChild(_surrenderButton);
        AddChild(UiTheme.PanelBox(turnRow));
    }

    private static HBoxContainer NewUnitRow(out ScrollContainer scroll)
    {
        var row = new HBoxContainer();
        row.AddThemeConstantOverride("separation", UiTheme.ElementSeparation);

        scroll = new ScrollContainer
        {
            SizeFlagsHorizontal = SizeFlags.ExpandFill,
            VerticalScrollMode = ScrollContainer.ScrollMode.Disabled,
        };
        scroll.AddChild(row);
        return row;
    }

    /// <summary>带标签的行：标签紧贴它标注的内容，而不是漂到别处。</summary>
    private static HBoxContainer LabelledRow(string label, Control content)
    {
        var row = new HBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        row.AddThemeConstantOverride("separation", UiTheme.ElementSeparation);

        var caption = UiTheme.Caption(label);
        caption.VerticalAlignment = VerticalAlignment.Center;
        row.AddChild(caption);

        content.SizeFlagsHorizontal = SizeFlags.ExpandFill;
        row.AddChild(content);
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
            if (_autoPlay)
            {
                GD.Print($"[riftcards] autoplay: {intent.Type} card={intent.CardInstanceId} target={intent.TargetId}");
            }
            _session?.SubmitCommand(intent.Type, intent.CardInstanceId, intent.TargetId);
        }
        Refresh();
    }

    private string? HeroIdOf(int seat) =>
        _board.View?.Players.FirstOrDefault(player => player.Seat == seat)?.HeroId;

    /// <summary>终局横幅：胜负与结束原因都来自服务端数据（投影的 winnerSeat 与 match_ended 载荷）。</summary>
    private void UpdateBanner()
    {
        var view = _board.View;
        if (view is null || !_board.IsFinished || view.WinnerSeat is null)
        {
            _banner.Visible = false;
            return;
        }

        var won = view.WinnerSeat == view.YouSeat;
        _bannerLabel.Text = $"{(won ? "胜利" : "败北")} · {EndReasonText()}";
        _bannerLabel.AddThemeColorOverride("font_color", won ? UiTheme.Accent : UiTheme.Danger);
        _banner.AddThemeStyleboxOverride("panel",
            UiTheme.PanelStyle(won ? UiTheme.Accent : UiTheme.Danger, Color.FromHtml("20262f")));
        _banner.Visible = true;
    }

    private string EndReasonText()
    {
        var ended = _session?.Events.LastOrDefault(gameEvent => gameEvent.Type == "match_ended");
        var reason = ended is null ? null : JsonText(ended.Data, "reason");
        return reason switch
        {
            "hero_defeated" => "英雄阵亡",
            "surrender" => "认输",
            null => "对局结束",
            _ => reason,
        };
    }

    private static string? JsonText(System.Text.Json.JsonElement data, string name) =>
        data.ValueKind == System.Text.Json.JsonValueKind.Object
        && data.TryGetProperty(name, out var value)
        && value.ValueKind == System.Text.Json.JsonValueKind.String
            ? value.GetString()
            : null;

    /// <summary>提示行：优先显示服务端错误码，其次是当前该做什么（终局时不再提示等待对手）。</summary>
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
            return "连接后点「进入匹配」。";
        }
        if (_board.IsFinished)
        {
            return "对局已结束。";
        }
        if (_board.AwaitingTarget)
        {
            return "请选择目标：任意单位或对手英雄。";
        }
        if (_board.Selection == SelectionKind.Attacker)
        {
            return "请选择攻击目标：对手单位或对手英雄。";
        }
        return _board.IsYourTurn
            ? "点击手牌出牌；点己方单位后选择攻击目标。"
            : "等待对手行动。";
    }

    private Color HintColor() =>
        _session?.LastCommandOutcome is { Accepted: false } || _session?.LastError is not null
            ? UiTheme.Danger
            : UiTheme.TextSecondary;

    /// <summary>
    /// 自动出牌一步：先出得起的牌（法术指向对手英雄），再攻击对手英雄，最后结束回合。
    /// 每个实例每回合只尝试一次，被服务端拒绝后不会死循环。
    /// </summary>
    private void AutoPlayStep()
    {
        var you = _board.You;
        var opponent = _board.Opponent;
        if (you is null || opponent is null)
        {
            return;
        }
        if (!_board.IsYourTurn)
        {
            // 回合已交给对手：清空本回合的尝试记录。
            _autoPlayAttempted.Clear();
            _autoPlayEndedTurn = false;
            return;
        }

        foreach (var card in you.Hand)
        {
            if (_autoPlayAttempted.Contains(card.InstanceId) || card.Cost > you.Energy)
            {
                continue;
            }
            _autoPlayAttempted.Add(card.InstanceId);
            var intent = _board.SelectHandCard(card.InstanceId);
            if (intent is not null)
            {
                Submit(intent);
                return;
            }
            if (_board.AwaitingTarget)
            {
                Submit(_board.SelectTarget(opponent.HeroId));
                return;
            }
        }

        foreach (var unit in you.Board)
        {
            if (_autoPlayAttempted.Contains(unit.InstanceId) || unit.AttacksRemaining <= 0)
            {
                continue;
            }
            _autoPlayAttempted.Add(unit.InstanceId);
            // 先选中攻击者，再指向对手英雄；两步都走真实意图接口。
            if (_board.SelectBoardUnit(unit.InstanceId) is null && _board.Selection == SelectionKind.Attacker)
            {
                Submit(_board.SelectHero(opponent.Seat));
                return;
            }
        }

        if (!_autoPlayEndedTurn)
        {
            _autoPlayEndedTurn = true;
            Submit(_board.EndTurn());
        }
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
