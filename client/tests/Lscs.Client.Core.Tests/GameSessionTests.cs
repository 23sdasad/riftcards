using Lscs.Client.Core.Protocol;
using Lscs.Client.Core.Session;

namespace Lscs.Client.Core.Tests;

public sealed class GameSessionTests
{
    [Fact]
    public void SendsHelloAndQueueAfterConnect()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);

        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();

        Assert.Equal(2, transport.Sent.Count);
        Assert.Contains("\"type\":\"hello\"", transport.Sent[0]);
        Assert.Contains("\"displayName\":\"Tester\"", transport.Sent[0]);
        Assert.Contains("\"type\":\"queue.join\"", transport.Sent[1]);
    }

    [Fact]
    public void AppliesMatchStartedSnapshot()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);
        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();

        transport.Receive("""
            {
              "type": "match.started",
              "data": {
                "matchId": "m_test",
                "seat": 1,
                "state": {
                  "matchId": "m_test",
                  "revision": 3,
                  "lastEventSeq": 7,
                  "turn": 2,
                  "activeSeat": 1,
                  "status": "active",
                  "winnerSeat": null,
                  "youSeat": 1,
                  "players": []
                }
              }
            }
            """);

        var view = Assert.IsType<MatchView>(session.View);
        Assert.Equal("m_test", view.MatchId);
        Assert.Equal(3, view.Revision);
        Assert.Equal(1, view.YouSeat);
    }

    private sealed class FakeTransport : IMessageTransport
    {
        public event Action? Connected;

        public event Action<string?>? Disconnected;

        public event Action<string>? TextReceived;

        public bool IsOpen { get; private set; }

        public List<string> Sent { get; } = [];

        public void Connect(string url)
        {
        }

        public void SendText(string text)
        {
            Sent.Add(text);
        }

        public void Open()
        {
            IsOpen = true;
            Connected?.Invoke();
        }

        public void Receive(string text)
        {
            TextReceived?.Invoke(text);
        }
    }
}
