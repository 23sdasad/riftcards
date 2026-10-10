using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;
using Xunit;

namespace Riftcards.Client.Core.Tests;

public sealed class EventPumpTests
{
    [Fact]
    public void TickRespectsPerTickLimit()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(2), Event(3)]);
        var presenter = new RecordingPresenter();
        var pump = new EventPump(queue, presenter, maxEventsPerTick: 2);

        Assert.Equal(2, pump.Tick());
        Assert.Equal([1L, 2L], presenter.Presented);
        Assert.Equal(1, queue.PendingCount);

        Assert.Equal(1, pump.Tick());
        Assert.Equal([1L, 2L, 3L], presenter.Presented);
        Assert.Equal(3, pump.PresentedCount);
    }

    [Fact]
    public void TickStopsWhenPresenterAsksToPause()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(2), Event(3)]);
        var presenter = new RecordingPresenter { StopAfterFirst = true };
        var pump = new EventPump(queue, presenter, maxEventsPerTick: 8);

        Assert.Equal(1, pump.Tick());

        // 事件已经出队（表现层拿到它），后续事件留在队列等待下次 Tick。
        Assert.Equal([1L], presenter.Presented);
        Assert.Equal(2, queue.PendingCount);

        presenter.StopAfterFirst = false;
        Assert.Equal(2, pump.Tick());
        Assert.Equal([1L, 2L, 3L], presenter.Presented);
    }

    [Fact]
    public void PauseBlocksTickUntilResume()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1)]);
        var presenter = new RecordingPresenter();
        var pump = new EventPump(queue, presenter);

        pump.Pause();
        Assert.Equal(0, pump.Tick());
        Assert.Empty(presenter.Presented);

        pump.Resume();
        Assert.Equal(1, pump.Tick());
        Assert.Equal([1L], presenter.Presented);
    }

    [Fact]
    public void SkipToSnapshotClearsQueueAndResetsPresenter()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(2), Event(3)]);
        var presenter = new RecordingPresenter();
        var pump = new EventPump(queue, presenter);
        var view = View(lastEventSeq: 3);

        pump.SkipToSnapshot(view);

        Assert.Equal(0, queue.PendingCount);
        Assert.Equal(3, queue.LastConsumedSeq);
        Assert.Equal(1, presenter.ResetCount);
        Assert.Equal(3, presenter.LastResetSeq);
    }

    [Fact]
    public void ResetClearsQueueAndStatistics()
    {
        var queue = new EventQueue();
        queue.Enqueue([Event(1), Event(1)]);
        var presenter = new RecordingPresenter();
        var pump = new EventPump(queue, presenter);
        pump.Tick();

        pump.Reset(View(lastEventSeq: 0));

        Assert.Equal(0, queue.PendingCount);
        Assert.Equal(0, queue.LastConsumedSeq);
        Assert.Equal(0, queue.DuplicateCount);
        Assert.Equal(0, pump.PresentedCount);
        Assert.Equal(1, presenter.ResetCount);
    }

    [Fact]
    public void RejectsInvalidPerTickLimit()
    {
        var queue = new EventQueue();
        var presenter = new RecordingPresenter();
        Assert.Throws<ArgumentOutOfRangeException>(() => new EventPump(queue, presenter, maxEventsPerTick: 0));
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

    private sealed class RecordingPresenter : IEventPresenter
    {
        public List<long> Presented { get; } = [];

        public bool StopAfterFirst { get; set; }

        public int ResetCount { get; private set; }

        public long LastResetSeq { get; private set; }

        public bool Present(GameEvent gameEvent)
        {
            Presented.Add(gameEvent.Seq);
            return !(StopAfterFirst && Presented.Count == 1);
        }

        public void Reset(MatchView? view)
        {
            ResetCount++;
            LastResetSeq = view?.LastEventSeq ?? 0;
        }
    }
}
