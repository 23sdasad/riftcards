using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;
using Xunit;

namespace Riftcards.Client.Core.Tests;

/// <summary>验证 GameSession 把协议接收、快照更新与表现事件消费分开。</summary>
public sealed class GameSessionEventQueueTests
{
    [Fact]
    public void MatchEventsFeedQueueWhileSnapshotBecomesAuthoritative()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);
        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();
        transport.Receive(MatchStarted("m_1"));

        transport.Receive(MatchEvents("m_1", baseRevision: 0, revision: 1, lastEventSeq: 2, seqs: [1, 2]));

        // 队列按 seq 待消费，快照立即成为权威状态，日志同时留档。
        Assert.Equal(2, session.Queue.PendingCount);
        Assert.Equal(2, session.View?.LastEventSeq);
        Assert.Equal(2, session.Events.Count);

        var presenter = new RecordingPresenter();
        var pump = new EventPump(session.Queue, presenter);
        pump.Tick();

        Assert.Equal([1L, 2L], presenter.Presented);
        Assert.Equal(0, session.Queue.PendingCount);
    }

    [Fact]
    public void RepeatingTheSameBatchDoesNotReplayPresentedEvents()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);
        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();
        transport.Receive(MatchStarted("m_1"));

        var batch = MatchEvents("m_1", baseRevision: 0, revision: 1, lastEventSeq: 2, seqs: [1, 2]);
        transport.Receive(batch);
        var presenter = new RecordingPresenter();
        var pump = new EventPump(session.Queue, presenter);
        pump.Tick();

        transport.Receive(batch);
        pump.Tick();

        Assert.Equal([1L, 2L], presenter.Presented);
        Assert.Equal(2, session.Queue.DuplicateCount);
    }

    [Fact]
    public void NewMatchResetsQueueCursorAndLog()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);
        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();
        transport.Receive(MatchStarted("m_1"));
        transport.Receive(MatchEvents("m_1", baseRevision: 0, revision: 1, lastEventSeq: 2, seqs: [1, 2]));

        transport.Receive(MatchStarted("m_2"));

        Assert.Equal(0, session.Queue.PendingCount);
        Assert.Equal(0, session.Queue.LastConsumedSeq);
        Assert.Empty(session.Events);
        Assert.Equal("m_2", session.View?.MatchId);

        // 新对局的 seq 从 1 重新开始，必须能被正常消费。
        transport.Receive(MatchEvents("m_2", baseRevision: 0, revision: 1, lastEventSeq: 1, seqs: [1]));
        var presenter = new RecordingPresenter();
        var pump = new EventPump(session.Queue, presenter);
        pump.Tick();
        Assert.Equal([1L], presenter.Presented);
    }

    private static string MatchStarted(string matchId) => $$"""
        {
          "type": "match.started",
          "data": {
            "matchId": "{{matchId}}",
            "seat": 0,
            "state": {
              "matchId": "{{matchId}}",
              "revision": 0,
              "lastEventSeq": 0,
              "turn": 1,
              "activeSeat": 0,
              "status": "active",
              "winnerSeat": null,
              "youSeat": 0,
              "players": []
            }
          }
        }
        """;

    private static string MatchEvents(string matchId, long baseRevision, long revision, long lastEventSeq, long[] seqs)
    {
        var events = string.Join(",", seqs.Select(seq => $$"""
            {
              "seq": {{seq}},
              "revision": {{revision}},
              "type": "damage_dealt",
              "actorSeat": 0,
              "visibility": "public",
              "data": { "targetId": "hero-1", "amount": 1, "hpAfter": 29 }
            }
            """));
        return $$"""
            {
              "type": "match.events",
              "data": {
                "matchId": "{{matchId}}",
                "baseRevision": {{baseRevision}},
                "revision": {{revision}},
                "lastEventSeq": {{lastEventSeq}},
                "events": [{{events}}],
                "state": {
                  "matchId": "{{matchId}}",
                  "revision": {{revision}},
                  "lastEventSeq": {{lastEventSeq}},
                  "turn": 1,
                  "activeSeat": 0,
                  "status": "active",
                  "winnerSeat": null,
                  "youSeat": 0,
                  "players": []
                }
              }
            }
            """;
    }

    private sealed class RecordingPresenter : IEventPresenter
    {
        public List<long> Presented { get; } = [];

        public bool Present(GameEvent gameEvent)
        {
            Presented.Add(gameEvent.Seq);
            return true;
        }

        public void Reset(MatchView? view)
        {
        }
    }
}
