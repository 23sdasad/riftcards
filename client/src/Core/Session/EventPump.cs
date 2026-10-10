using Riftcards.Client.Core.Protocol;

namespace Riftcards.Client.Core.Session;

/// <summary>
/// 表现层适配接口：由具体 UI（例如 Godot）实现，Core 只负责顺序与校正。
/// 实现方不得改变规则状态，也不得阻塞快照校正。
/// </summary>
public interface IEventPresenter
{
    /// <summary>
    /// 播放一个事件。返回 false 表示本帧暂停继续消费（例如动画尚未播完）；
    /// 事件已经出队，因此实现方应在合适时机调用 <see cref="EventPump.SkipToSnapshot"/> 收敛。
    /// </summary>
    bool Present(GameEvent gameEvent);

    /// <summary>队列被清空、校正或开始新对局时调用，让表现层回到最新快照。</summary>
    void Reset(MatchView? view);
}

/// <summary>
/// 事件消费泵：按帧从 <see cref="EventQueue"/> 取事件交给表现层。
///
/// 每帧播放数量有上限，长队列不会卡住主循环；表现层返回 false 时本帧立即停止。
/// 权威状态始终来自最新 <see cref="MatchView"/>，因此表现层落后不会影响对局。
/// </summary>
public sealed class EventPump
{
    /// <summary>默认每帧最多播放的事件数。</summary>
    public const int DefaultMaxEventsPerTick = 4;

    private readonly EventQueue _queue;
    private readonly IEventPresenter _presenter;

    public EventPump(EventQueue queue, IEventPresenter presenter, int maxEventsPerTick = DefaultMaxEventsPerTick)
    {
        _queue = queue ?? throw new ArgumentNullException(nameof(queue));
        _presenter = presenter ?? throw new ArgumentNullException(nameof(presenter));
        MaxEventsPerTick = maxEventsPerTick > 0
            ? maxEventsPerTick
            : throw new ArgumentOutOfRangeException(nameof(maxEventsPerTick), maxEventsPerTick, "每帧上限必须为正数");
    }

    /// <summary>每帧最多播放的事件数。</summary>
    public int MaxEventsPerTick { get; set; }

    /// <summary>累计播放的事件数量。</summary>
    public int PresentedCount { get; private set; }

    /// <summary>队列是否暂停消费。</summary>
    public bool IsPaused => _queue.IsPaused;

    /// <summary>消费至多 <see cref="MaxEventsPerTick"/> 个事件；返回本帧实际播放数量。</summary>
    public int Tick()
    {
        var played = 0;
        while (played < MaxEventsPerTick && _queue.TryDequeue(out var gameEvent))
        {
            played++;
            PresentedCount++;
            if (!_presenter.Present(gameEvent))
            {
                break;
            }
        }
        return played;
    }

    /// <summary>暂停消费；事件继续入队，不会丢失。</summary>
    public void Pause() => _queue.Pause();

    /// <summary>恢复消费。</summary>
    public void Resume() => _queue.Resume();

    /// <summary>立即收敛到最新快照：清空待播放事件并让表现层重置。</summary>
    public void SkipToSnapshot(MatchView? view)
    {
        _queue.Clear();
        _queue.CorrectFrom(view);
        _presenter.Reset(view);
    }

    /// <summary>开始新对局：清空队列与统计，并让表现层重置。</summary>
    public void Reset(MatchView? view)
    {
        _queue.Reset();
        PresentedCount = 0;
        _presenter.Reset(view);
    }
}
