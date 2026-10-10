using Godot;
using Riftcards.Client.Core.Session;

namespace Riftcards.Client.Godot.Match;

/// <summary>
/// 事件面板：拥有事件泵与表现适配器，每帧消费队列，并提供“跳过动画”立即收敛到最新快照。
///
/// 对局面板始终渲染最新快照，因此这里的播放只是表现层补充；队列积压或表现失败都不会影响对局。
/// </summary>
public sealed partial class EventLogPanel : VBoxContainer
{
    private RichTextLabel _log = null!;
    private Label _summary = null!;
    private GameSession? _session;
    private EventPump? _pump;
    private string? _matchId;

    public override void _Ready()
    {
        AddThemeConstantOverride("separation", 4);

        var header = new HBoxContainer();
        header.AddThemeConstantOverride("separation", 8);
        AddChild(header);

        header.AddChild(new Label { Text = "事件" });

        var skipButton = new Button { Text = "跳过动画" };
        skipButton.Pressed += Skip;
        header.AddChild(skipButton);

        _summary = new Label();
        header.AddChild(_summary);

        _log = new RichTextLabel
        {
            FitContent = false,
            ScrollActive = true,
            CustomMinimumSize = new Vector2(0, 160),
        };
        AddChild(_log);
    }

    /// <summary>绑定会话并创建事件泵；重复调用会替换旧泵。</summary>
    public void Bind(GameSession session)
    {
        if (_session is not null)
        {
            _session.Changed -= OnSessionChanged;
        }
        _session = session;
        _pump = new EventPump(session.Queue, new EventLogPresenter(_log));
        _matchId = null;
        session.Changed += OnSessionChanged;
        OnSessionChanged();
    }

    public override void _Process(double delta)
    {
        _pump?.Tick();
        UpdateSummary();
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
        var text = $"待播放 {queue.PendingCount}｜已消费 seq {queue.LastConsumedSeq}";
        if (queue.HasGap)
        {
            text += $"｜缺帧（下一个 {queue.NextMissingSeq}）";
        }
        if (queue.UnknownCount > 0)
        {
            text += $"｜未知事件 {queue.UnknownCount}";
        }
        _summary.Text = text;
    }
}
