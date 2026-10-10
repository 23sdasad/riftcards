using System.Text.Json;
using Riftcards.Client.Core.Protocol;
using Xunit;

namespace Riftcards.Client.Core.Tests;

public sealed class ProtocolJsonTests
{
    [Fact]
    public void SerializesClientMessagesWithCamelCase()
    {
        var message = new ClientEnvelope
        {
            Type = ProtocolNames.MatchCommand,
            RequestId = "req-1",
            Data = new MatchCommandRequest
            {
                MatchId = "m_test",
                CommandId = "cmd-1",
                ExpectedRevision = 4,
                Command = new PlayerCommand
                {
                    Type = ProtocolNames.EndTurn,
                },
            },
        };

        var json = ProtocolJson.Serialize(message);

        Assert.Contains("\"type\":\"match.command\"", json);
        Assert.Contains("\"expectedRevision\":4", json);
    }

    [Fact]
    public void ParsesMatchStartedProjection()
    {
        const string json = """
            {
              "type": "match.started",
              "data": {
                "matchId": "m_test",
                "seat": 0,
                "state": {
                  "matchId": "m_test",
                  "revision": 0,
                  "lastEventSeq": 0,
                  "turn": 1,
                  "activeSeat": 0,
                  "status": "active",
                  "winnerSeat": null,
                  "youSeat": 0,
                  "players": [
                    {
                      "seat": 0,
                      "heroId": "hero-0",
                      "hp": 30,
                      "energy": 1,
                      "turnNumber": 1,
                      "deckCount": 26,
                      "handCount": 4,
                      "hand": [],
                      "board": []
                    },
                    {
                      "seat": 1,
                      "heroId": "hero-1",
                      "hp": 30,
                      "energy": 0,
                      "turnNumber": 0,
                      "deckCount": 26,
                      "handCount": 4,
                      "hand": [],
                      "board": []
                    }
                  ]
                }
              }
            }
            """;

        var envelope = ProtocolJson.Deserialize<ServerEnvelope>(json);
        var started = envelope?.Data.Deserialize<MatchStartedData>(ProtocolJson.Options);
        var startedValue = Assert.IsType<MatchStartedData>(started);

        Assert.Equal("m_test", startedValue.MatchId);
        Assert.Equal(0, startedValue.Seat);
        Assert.Equal(30, startedValue.State.Players[0].Hp);
        Assert.Empty(startedValue.State.Players[1].Hand);
        Assert.Equal(4, startedValue.State.Players[1].HandCount);
    }

    [Fact]
    public void ParsesCommandResultEnvelopeWithRequestId()
    {
        const string json = """
            {
              "type": "match.command_result",
              "requestId": "req-command",
              "data": {
                "commandId": "cmd-001",
                "accepted": false,
                "revision": 4,
                "error": {
                  "code": "insufficient_energy",
                  "message": "card costs 2 energy, 1 available"
                }
              }
            }
            """;

        var envelope = ProtocolJson.Deserialize<ServerEnvelope>(json);
        var envelopeValue = Assert.IsType<ServerEnvelope>(envelope);
        var result = Assert.IsType<CommandResultData>(
            envelopeValue.Data.Deserialize<CommandResultData>(ProtocolJson.Options));

        Assert.Equal("req-command", envelopeValue.RequestId);
        Assert.Equal("cmd-001", result.CommandId);
        Assert.False(result.Accepted);
        Assert.Equal(4, result.Revision);
        Assert.Equal("insufficient_energy", result.Error?.Code);
    }

    [Fact]
    public void ParsesBroadcastEnvelopeWithoutRequestId()
    {
        const string json = """
            {
              "type": "match.events",
              "data": {
                "matchId": "m_test",
                "baseRevision": 4,
                "revision": 5,
                "lastEventSeq": 9,
                "events": [],
                "state": {
                  "matchId": "m_test",
                  "revision": 5,
                  "lastEventSeq": 9,
                  "turn": 2,
                  "activeSeat": 1,
                  "status": "active",
                  "winnerSeat": null,
                  "youSeat": 0,
                  "players": []
                }
              }
            }
            """;

        var envelope = ProtocolJson.Deserialize<ServerEnvelope>(json);
        var envelopeValue = Assert.IsType<ServerEnvelope>(envelope);
        var events = Assert.IsType<MatchEventsData>(
            envelopeValue.Data.Deserialize<MatchEventsData>(ProtocolJson.Options));

        Assert.Null(envelopeValue.RequestId);
        Assert.Equal(4, events.BaseRevision);
        Assert.Equal(5, events.Revision);
        Assert.Equal(5, events.State.Revision);
    }
}
