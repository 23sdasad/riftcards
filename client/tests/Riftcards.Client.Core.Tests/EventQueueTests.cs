using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;
using Xunit;

namespace Riftcards.Client.Core.Tests;

public sealed class EventQueueTests
{
    [Fact]
    public void ConsumesOutOfOrderBatchBySeq()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(3), Event(1), Event(2)]);

        Assert.Equal([1L, 2L, 3L], Drain(queue));
    }

    [Fact]
    public void DropsDuplicatesOfConsumedEvents()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(2)]);
        Drain(queue);

        queue.Enqueue([Event(1), Event(2)]);

        Assert.Equal(0, queue.PendingCount);
        Assert.Equal(2, queue.DuplicateCount);
    }

    [Fact]
    public void DropsDuplicatesStillPending()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(1)]);

        Assert.Equal(1, queue.PendingCount);
        Assert.Equal(1, queue.DuplicateCount);
    }

    [Fact]
    public void AcceptsLateArrivalInOrder()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(2)]);
        Assert.Equal([1L, 2L], Drain(queue));

        queue.Enqueue([Event(4)]);
        queue.Enqueue([Event(3)]);

        Assert.Equal([3L, 4L], Drain(queue));
        Assert.Equal(1, queue.OutOfOrderCount);
    }

    [Fact]
    public void RecordsGapWithoutBlockingConsumption()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(3)]);

        Assert.True(queue.HasGap);
        Assert.Equal(2, queue.NextMissingSeq);
        // 缺帧不阻塞：已有事件照常消费，权威状态由快照负责（补帧属于 Phase 2）。
        Assert.Equal([1L, 3L], Drain(queue));
    }

    [Fact]
    public void DeliversUnknownEventTypesButCountsThem()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1, "future_event")]);

        Assert.Equal(1, queue.UnknownCount);
        Assert.Equal([1L], Drain(queue));
    }

    [Fact]
    public void PauseBlocksConsumptionWithoutLosingEvents()
    {
        var queue = new EventQueue();
        queue.Pause();
        queue.Enqueue([Event(1), Event(2)]);

        Assert.False(queue.TryDequeue(out _));
        Assert.Equal(2, queue.PendingCount);

        queue.Resume();
        Assert.Equal([1L, 2L], Drain(queue));
    }

    [Fact]
    public void ClearDropsPendingEventsAndCountsThem()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(2)]);

        queue.Clear();

        Assert.Equal(0, queue.PendingCount);
        Assert.Equal(2, queue.DroppedCount);
        Assert.False(queue.HasGap);
    }

    [Fact]
    public void CorrectFromSkipsEventsCoveredBySnapshot()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(2), Event(3), Event(4), Event(5)]);

        queue.CorrectFrom(View(lastEventSeq: 3));

        // 校正把游标推进到快照位置，被覆盖的事件不再播放。
        Assert.Equal(3, queue.LastConsumedSeq);
        Assert.Equal(3, queue.DroppedCount);
        Assert.Equal([4L, 5L], Drain(queue));

        // 校正后到达的过期事件按重复处理。
        queue.Enqueue([Event(2)]);
        Assert.Equal(0, queue.PendingCount);
        Assert.Equal(1, queue.DuplicateCount);
    }

    [Fact]
    public void CorrectFromKeepsEventsNewerThanSnapshot()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(4), Event(5)]);

        queue.CorrectFrom(View(lastEventSeq: 2));

        Assert.Equal(2, queue.PendingCount);
        Assert.Equal(0, queue.DroppedCount);
    }

    [Fact]
    public void CapacityDropsOldestEvents()
    {
        var queue = new EventQueue(capacity: 2);
        queue.Enqueue([Event(1), Event(2), Event(3)]);

        Assert.Equal(2, queue.PendingCount);
        Assert.Equal(1, queue.OverflowCount);
        Assert.Equal([2L, 3L], Drain(queue));
    }

    [Fact]
    public void ResetClearsCursorAndStatistics()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(1), Event(2)]);
        Drain(queue);
        queue.Enqueue([Event(3)]);

        queue.Reset();

        Assert.Equal(0, queue.PendingCount);
        Assert.Equal(0, queue.LastConsumedSeq);
        Assert.Equal(0, queue.DuplicateCount);
        Assert.False(queue.HasGap);

        // 新对局的 seq 从 1 重新开始，不会被当成重复。
        queue.Enqueue([Event(1)]);
        Assert.Equal(1, queue.PendingCount);
    }

    [Fact]
    public void RejectsNonPositiveCapacity()
    {
        Assert.Throws<ArgumentOutOfRangeException>(() => new EventQueue(0));
    }

    private static List<long> Drain(EventQueue queue)
    {
        var seqs = new List<long>();
        while (queue.TryDequeue(out var gameEvent))
        {
            seqs.Add(gameEvent.Seq);
        }
        return seqs;
    }

    private static GameEvent Event(long seq, string type = "damage_dealt") =>
        new()
        {
            Seq = seq,
            Revision = 1,
            Type = type,
            ActorSeat = 0,
            Visibility = "public",
        };

    private static MatchView View(long lastEventSeq) =>
        new()
        {
            MatchId = "m_test",
            Revision = 1,
            LastEventSeq = lastEventSeq,
            Turn = 1,
            ActiveSeat = 0,
            Status = "active",
            YouSeat = 0,
            Players = [],
        };
}
