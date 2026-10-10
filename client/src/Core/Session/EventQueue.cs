using Riftcards.Client.Core.Protocol;

namespace Riftcards.Client.Core.Session;

/// <summary>
/// 按 <c>seq</c> 顺序消费领域事件的事件队列。
///
/// 职责与边界：
///   - 只负责顺序、去重、缺帧记录与容量控制，不解释事件含义，也不改变规则状态。
///   - 表现层通过 <see cref="EventPump"/> 消费；队列本身不引用任何 UI 或 Godot 类型。
///   - 权威状态来自 <see cref="MatchView"/> 快照：队列落后时用 <see cref="CorrectFrom"/> 收敛，
///     丢弃快照已覆盖的事件，因此动画不会阻塞对局。
/// </summary>
public sealed class EventQueue
{
    /// <summary>默认容量：超出后丢弃最旧的事件（快照仍然权威）。</summary>
    public const int DefaultCapacity = 256;

    private readonly List<GameEvent> _pending = [];
    private long _lastConsumedSeq;
    private long _lastEnqueuedSeq;

    public EventQueue(int capacity = DefaultCapacity)
    {
        if (capacity <= 0)
        {
            throw new ArgumentOutOfRangeException(nameof(capacity), capacity, "容量必须为正数");
        }
        Capacity = capacity;
    }

    /// <summary>队列容量上限。</summary>
    public int Capacity { get; }

    /// <summary>是否已暂停消费（入队不受影响）。</summary>
    public bool IsPaused { get; private set; }

    /// <summary>待消费事件数量。</summary>
    public int PendingCount => _pending.Count;

    /// <summary>已消费（或已被快照校正跳过）的最大 <c>seq</c>。</summary>
    public long LastConsumedSeq => _lastConsumedSeq;

    /// <summary>是否存在缺帧：队列最前面的 <c>seq</c> 大于期望的下一个序号。</summary>
    public bool HasGap { get; private set; }

    /// <summary>缺帧时期望的下一个序号；无缺帧时为 0。</summary>
    public long NextMissingSeq { get; private set; }

    /// <summary>已丢弃的重复事件数量。</summary>
    public int DuplicateCount { get; private set; }

    /// <summary>迟到但仍可消费的事件数量（乱序到达）。</summary>
    public int OutOfOrderCount { get; private set; }

    /// <summary>未知事件类型数量；这些事件仍会被投递，由表现层忽略。</summary>
    public int UnknownCount { get; private set; }

    /// <summary>被清空或校正丢弃的事件数量。</summary>
    public int DroppedCount { get; private set; }

    /// <summary>因容量上限被丢弃的最旧事件数量。</summary>
    public int OverflowCount { get; private set; }

    /// <summary>
    /// 入队一批事件。批内按 <c>seq</c> 排序；与已消费或已在队列中的序号重复的事件被丢弃。
    /// </summary>
    public void Enqueue(IEnumerable<GameEvent> events)
    {
        ArgumentNullException.ThrowIfNull(events);

        var batch = events.Where(gameEvent => gameEvent is not null).OrderBy(gameEvent => gameEvent.Seq);
        foreach (var gameEvent in batch)
        {
            if (gameEvent.Seq <= _lastConsumedSeq || _pending.Any(pending => pending.Seq == gameEvent.Seq))
            {
                DuplicateCount++;
                continue;
            }
            if (!ProtocolNames.KnownEventTypes.Contains(gameEvent.Type))
            {
                UnknownCount++;
            }
            if (gameEvent.Seq < _lastEnqueuedSeq)
            {
                // 迟到但尚未消费：仍然按序插入，保证消费顺序正确。
                OutOfOrderCount++;
            }
            _pending.Add(gameEvent);
            if (gameEvent.Seq > _lastEnqueuedSeq)
            {
                _lastEnqueuedSeq = gameEvent.Seq;
            }
        }

        _pending.Sort((left, right) => left.Seq.CompareTo(right.Seq));
        UpdateGap();
        TrimToCapacity();
    }

    /// <summary>取下一个事件；暂停中或队列为空时返回 false。</summary>
    public bool TryDequeue(out GameEvent gameEvent)
    {
        gameEvent = null!;
        if (IsPaused || _pending.Count == 0)
        {
            return false;
        }

        gameEvent = _pending[0];
        _pending.RemoveAt(0);
        if (gameEvent.Seq > _lastConsumedSeq)
        {
            _lastConsumedSeq = gameEvent.Seq;
        }
        UpdateGap();
        return true;
    }

    /// <summary>暂停消费；入队继续，不会丢事件。</summary>
    public void Pause() => IsPaused = true;

    /// <summary>恢复消费。</summary>
    public void Resume() => IsPaused = false;

    /// <summary>清空待消费事件，保留游标与统计。</summary>
    public void Clear()
    {
        DroppedCount += _pending.Count;
        _pending.Clear();
        UpdateGap();
    }

    /// <summary>
    /// 按最新快照校正：丢弃快照已经覆盖的事件，并把游标推进到快照位置。
    /// 用于“跳过动画”或队列落后太多时立即收敛，避免回放过期表现。
    /// </summary>
    public void CorrectFrom(MatchView? view)
    {
        if (view is null)
        {
            Clear();
            return;
        }

        var covered = view.LastEventSeq;
        if (covered <= _lastConsumedSeq)
        {
            return;
        }

        DroppedCount += _pending.Count(gameEvent => gameEvent.Seq <= covered);
        _pending.RemoveAll(gameEvent => gameEvent.Seq <= covered);
        _lastConsumedSeq = covered;
        UpdateGap();
    }

    /// <summary>开始新对局：清空队列、游标与统计。</summary>
    public void Reset()
    {
        _pending.Clear();
        _lastConsumedSeq = 0;
        _lastEnqueuedSeq = 0;
        HasGap = false;
        NextMissingSeq = 0;
        DuplicateCount = 0;
        OutOfOrderCount = 0;
        UnknownCount = 0;
        DroppedCount = 0;
        OverflowCount = 0;
    }

    /// <summary>
    /// 重算缺帧状态：从已消费游标开始扫描整个待处理列表，找到第一个不连续的位置。
    /// 只检查队首是不够的——中间缺一个序号同样需要被记录。
    /// </summary>
    private void UpdateGap()
    {
        HasGap = false;
        NextMissingSeq = 0;
        if (_pending.Count == 0)
        {
            return;
        }

        var expected = _lastConsumedSeq + 1;
        foreach (var gameEvent in _pending)
        {
            if (gameEvent.Seq > expected)
            {
                HasGap = true;
                NextMissingSeq = expected;
                return;
            }
            expected = gameEvent.Seq + 1;
        }
    }

    private void TrimToCapacity()
    {
        while (_pending.Count > Capacity)
        {
            _pending.RemoveAt(0);
            OverflowCount++;
            DroppedCount++;
        }
    }
}
