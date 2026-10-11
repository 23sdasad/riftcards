using Godot;
using Riftcards.Client.Core.Session;

namespace Riftcards.Client.Godot.Match;

/// <summary>
/// 事件面板：可读事件文本、待播放数量与缺帧提示，可折叠，并支持“跳过动画”立即收敛到最新快照。
///
/// 事件面板是次要信息：固定小高度、可折叠，不挤占对局区（见 ai-docs/architecture/client-ui.md）。
/// 对局面板始终渲染最新快照，因此这里积压或失败都不会影响对局。
/// </summary>
public sealed partial class EventLogPanel : VBoxContainer
{
    private RichTextLabel _log = null!;
    private Label _summary = null!;
    private Button _toggleButton = null!;
    private GameSession? _session;
    private EventPump? _pump;
    private string? _matchId;
    private bool _collapsed;
    private bool _collapsedByHeight;
    private bool _wasSmall;

    public override void _Ready()
    {
        AddThemeConstantOverride("separation", UiTheme.ElementSeparation);

        var body = new VBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        body.AddThemeConstantOverride("separation", UiTheme.ElementSeparation);

        var header = new HBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        header.AddThemeConstantOverride("separation", UiTheme.ElementSeparation);
        header.AddChild(UiTheme.Heading("事件"));

        _summary = UiTheme.Caption("待播放 0");
        _summary.SizeFlagsHorizontal = SizeFlags.ExpandFill;
        _summary.VerticalAlignment = VerticalAlignment.Center;
        header.AddChild(_summary);

        _toggleButton = new Button { Text = "折叠" };
        UiTheme.StyleButton(_toggleButton);
        _toggleButton.Pressed += ToggleCollapsed;
        header.AddChild(_toggleButton);

        var skipButton = new Button { Text = "跳过动画" };
        UiTheme.StyleButton(skipButton, UiTheme.Accent);
        skipButton.Pressed += Skip;
        header.AddChild(skipButton);

        body.AddChild(header);

        _log = new RichTextLabel
        {
            FitContent = false,
            ScrollActive = true,
            ScrollFollowing = true,
            CustomMinimumSize = new Vector2(0, UiTheme.EventPanelHeight),
            SizeFlagsHorizontal = SizeFlags.ExpandFill,
        };
        _log.AddThemeFontSizeOverride("normal_font_size", UiTheme.FontCaption);
        _log.AddThemeColorOverride("default_color", UiTheme.TextSecondary);
        body.AddChild(_log);

        AddChild(UiTheme.PanelBox(body));
    }

    /// <summary>绑定会话并创建事件泵；重复调用会替换旧泵。</summary>
    public void Bind(GameSession session)
    {
        if (_session is not null)
        {
            _session.Changed -= OnSessionChanged;
        }
        _session = session;
        _pump = new EventPump(session.Queue, new EventLogPresenter(_log, session));
        _matchId = null;
        session.Changed += OnSessionChanged;
        OnSessionChanged();
    }

    public override void _Process(double delta)
    {
        _pump?.Tick();
        UpdateSummary();
        UpdateCollapseForHeight();
    }

    /// <summary>
    /// 窗口高度不足时默认折叠事件面板，把纵向空间让给对局区；
    /// 只在高度跨越阈值时改变状态，因此玩家手动展开后不会被立刻折叠回去。
    /// </summary>
    private void UpdateCollapseForHeight()
    {
        var small = GetViewportRect().Size.Y < 620f;
        if (small == _wasSmall)
        {
            return;
        }
        _wasSmall = small;
        _collapsedByHeight = small;
        ApplyCollapsed(_collapsed || _collapsedByHeight);
    }

    private void ToggleCollapsed()
    {
        if (_collapsed || _collapsedByHeight)
        {
            // 手动展开优先于按高度自动折叠。
            _collapsed = false;
            _collapsedByHeight = false;
        }
        else
        {
            _collapsed = true;
        }
        ApplyCollapsed(_collapsed || _collapsedByHeight);
    }

    private void ApplyCollapsed(bool collapsed)
    {
        _log.Visible = !collapsed;
        _toggleButton.Text = collapsed ? "展开" : "折叠";
    }

    private void Skip() => _pump?.SkipToSnapshot(_session?.View);

    private void OnSessionChanged()
    {
        var view = _session?.View;
        if (view?.MatchId == _matchId)
        {
            return;
        }
        // 新对局：清空队列与统计，让表现层从新快照开始。
        _matchId = view?.MatchId;
        _pump?.Reset(view);
    }

    private void UpdateSummary()
    {
        if (_session is null)
        {
            _summary.Text = string.Empty;
            return;
        }

        var queue = _session.Queue;
        var text = $"待播放 {queue.PendingCount} │ 已消费 seq {queue.LastConsumedSeq}";
        if (queue.HasGap)
        {
            text += $" │ 缺帧（下一个 {queue.NextMissingSeq}）";
        }
        if (queue.UnknownCount > 0)
        {
            text += $" │ 未知事件 {queue.UnknownCount}";
        }
        _summary.Text = text;
    }
}
