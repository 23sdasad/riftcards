using System.Text.Json;
using Lscs.Client.Core.Protocol;

namespace Lscs.Client.Core.Tests;

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
}
