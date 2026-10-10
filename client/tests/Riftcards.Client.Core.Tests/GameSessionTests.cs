using System.Text.Json;
using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;
using Xunit;

namespace Riftcards.Client.Core.Tests;

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
    public void EverySentMessageCarriesRequestId()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);

        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();
        session.Ping();

        Assert.NotEmpty(transport.Sent);
        Assert.All(transport.Sent, envelope => Assert.False(string.IsNullOrEmpty(RequestIdOf(envelope))));
    }

    [Fact]
    public void AppliesMatchStartedSnapshot()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);
        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();

        transport.Receive(MatchStartedJson);

        var view = Assert.IsType<MatchView>(session.View);
        Assert.Equal("m_test", view.MatchId);
        Assert.Equal(3, view.Revision);
        Assert.Equal(1, view.YouSeat);
    }

    [Fact]
    public void CorrelatesCommandResultWithOriginalRequest()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);
        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();
        CompleteHandshake(transport);
        Assert.Equal(0, session.PendingRequests);
        transport.Receive(MatchStartedJson);

        session.SubmitCommand(ProtocolNames.EndTurn);
        var commandEnvelope = transport.Sent[^1];
        var requestId = RequestIdOf(commandEnvelope);
        var commandId = CommandIdOf(commandEnvelope);
        Assert.Equal(1, session.PendingRequests);

        transport.Receive($$"""
            {
              "type": "match.command_result",
              "requestId": "{{requestId}}",
              "data": {
                "commandId": "{{commandId}}",
                "accepted": true,
                "revision": 4,
                "error": null
              }
            }
            """);

        var outcome = Assert.IsType<CommandOutcome>(session.LastCommandOutcome);
        Assert.Equal(requestId, outcome.RequestId);
        Assert.Equal(commandId, outcome.CommandId);
        Assert.True(outcome.Accepted);
        Assert.Equal(4, outcome.Revision);
        Assert.Null(outcome.ErrorCode);
        Assert.Equal(0, session.PendingRequests);
        Assert.Equal(requestId, session.LastResponseRequestId);
    }

    [Fact]
    public void CorrelatesRejectedCommandResultWithOriginalRequest()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);
        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();
        transport.Receive(MatchStartedJson);

        session.SubmitCommand(ProtocolNames.EndTurn);
        var requestId = RequestIdOf(transport.Sent[^1]);

        transport.Receive($$"""
            {
              "type": "match.command_result",
              "requestId": "{{requestId}}",
              "data": {
                "commandId": "cmd-1",
                "accepted": false,
                "revision": 3,
                "error": { "code": "stale_revision", "message": "expected revision 2, current revision is 3" }
              }
            }
            """);

        var outcome = Assert.IsType<CommandOutcome>(session.LastCommandOutcome);
        Assert.Equal(requestId, outcome.RequestId);
        Assert.False(outcome.Accepted);
        Assert.Equal("stale_revision", outcome.ErrorCode);
        Assert.Equal(3, outcome.Revision);
    }

    [Fact]
    public void CorrelatesErrorResponseWithOriginalRequest()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);
        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();
        CompleteHandshake(transport);

        session.JoinQueue();
        var requestId = RequestIdOf(transport.Sent[^1]);
        Assert.Equal(1, session.PendingRequests);

        transport.Receive($$"""
            {
              "type": "error",
              "requestId": "{{requestId}}",
              "data": { "code": "not_in_match", "message": "session is not in a match" }
            }
            """);

        Assert.Equal(requestId, session.LastErrorRequestId);
        Assert.Equal("not_in_match", session.LastError?.Code);
        Assert.Equal(0, session.PendingRequests);
    }

    [Fact]
    public void BroadcastDoesNotChangeResponseCorrelation()
    {
        var transport = new FakeTransport();
        var session = new GameSession(transport);
        session.Connect("ws://127.0.0.1:8080/ws", "Tester");
        transport.Open();

        session.JoinQueue();
        var requestId = RequestIdOf(transport.Sent[^1]);
        transport.Receive($$"""
            {
              "type": "queue.status",
              "requestId": "{{requestId}}",
              "data": { "state": "waiting", "position": 1 }
            }
            """);
        Assert.Equal(requestId, session.LastResponseRequestId);

        // 广播不带 requestId，不应影响已记录的关联。
        transport.Receive("""
            {
              "type": "match.events",
              "data": {
                "matchId": "m_test",
                "baseRevision": 3,
                "revision": 4,
                "lastEventSeq": 5,
                "events": [],
                "state": {
                  "matchId": "m_test",
                  "revision": 4,
                  "lastEventSeq": 5,
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

        Assert.Equal(requestId, session.LastResponseRequestId);
        Assert.Null(session.LastCommandOutcome);
    }

    private const string MatchStartedJson = """
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
        """;

    private static string RequestIdOf(string envelope)
    {
        using var document = JsonDocument.Parse(envelope);
        return document.RootElement.GetProperty("requestId").GetString() ?? string.Empty;
    }

    // CompleteHandshake 应答连接后自动发出的 hello 与 queue.join，
    // 让后续断言只关注被测请求的待处理数量。
    private static void CompleteHandshake(FakeTransport transport)
    {
        transport.Receive($$"""
            {
              "type": "welcome",
              "requestId": "{{RequestIdOf(transport.Sent[0])}}",
              "data": {
                "connectionId": "c1",
                "playerId": "p1",
                "displayName": "Tester",
                "serverVersion": "0.1.0",
                "protocolVersion": "0.1"
              }
            }
            """);
        transport.Receive($$"""
            {
              "type": "queue.status",
              "requestId": "{{RequestIdOf(transport.Sent[1])}}",
              "data": { "state": "waiting", "position": 1 }
            }
            """);
    }

    private static string CommandIdOf(string envelope)
    {
        using var document = JsonDocument.Parse(envelope);
        return document.RootElement.GetProperty("data").GetProperty("commandId").GetString() ?? string.Empty;
    }
}
