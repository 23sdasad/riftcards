using System.Text.Json;
using System.Text.Json.Serialization;

namespace Riftcards.Client.Core.Protocol;

public sealed class ClientEnvelope
{
    [JsonPropertyName("type")]
    public string Type { get; init; } = string.Empty;

    [JsonPropertyName("requestId")]
    public string? RequestId { get; init; }

    [JsonPropertyName("data")]
    public object? Data { get; init; }
}

public sealed class ServerEnvelope
{
    [JsonPropertyName("type")]
    public string Type { get; init; } = string.Empty;

    [JsonPropertyName("requestId")]
    public string? RequestId { get; init; }

    [JsonPropertyName("data")]
    public JsonElement Data { get; init; }
}

public sealed class HelloData
{
    [JsonPropertyName("protocolVersion")]
    public string ProtocolVersion { get; init; } = ProtocolNames.Version;

    [JsonPropertyName("clientVersion")]
    public string ClientVersion { get; init; } = "0.1.0";

    [JsonPropertyName("displayName")]
    public string DisplayName { get; init; } = "Player";
}

public sealed class WelcomeData
{
    [JsonPropertyName("connectionId")]
    public string ConnectionId { get; init; } = string.Empty;

    [JsonPropertyName("playerId")]
    public string PlayerId { get; init; } = string.Empty;

    [JsonPropertyName("displayName")]
    public string DisplayName { get; init; } = string.Empty;

    [JsonPropertyName("serverVersion")]
    public string ServerVersion { get; init; } = string.Empty;

    [JsonPropertyName("protocolVersion")]
    public string ProtocolVersion { get; init; } = string.Empty;
}

public sealed class QueueStatusData
{
    [JsonPropertyName("state")]
    public string State { get; init; } = string.Empty;

    [JsonPropertyName("position")]
    public int Position { get; init; }
}

public sealed class PlayerCommand
{
    [JsonPropertyName("type")]
    public string Type { get; init; } = string.Empty;

    [JsonPropertyName("cardInstanceId")]
    public string? CardInstanceId { get; init; }

    [JsonPropertyName("targetId")]
    public string? TargetId { get; init; }
}

public sealed class MatchCommandRequest
{
    [JsonPropertyName("matchId")]
    public string MatchId { get; init; } = string.Empty;

    [JsonPropertyName("commandId")]
    public string CommandId { get; init; } = string.Empty;

    [JsonPropertyName("expectedRevision")]
    public long ExpectedRevision { get; init; }

    [JsonPropertyName("command")]
    public PlayerCommand Command { get; init; } = new();
}

public sealed class MatchStartedData
{
    [JsonPropertyName("matchId")]
    public string MatchId { get; init; } = string.Empty;

    [JsonPropertyName("seat")]
    public int Seat { get; init; }

    [JsonPropertyName("state")]
    public MatchView State { get; init; } = new();
}

public sealed class MatchEventsData
{
    [JsonPropertyName("matchId")]
    public string MatchId { get; init; } = string.Empty;

    [JsonPropertyName("baseRevision")]
    public long BaseRevision { get; init; }

    [JsonPropertyName("revision")]
    public long Revision { get; init; }

    [JsonPropertyName("lastEventSeq")]
    public long LastEventSeq { get; init; }

    [JsonPropertyName("events")]
    public List<GameEvent> Events { get; init; } = [];

    [JsonPropertyName("state")]
    public MatchView State { get; init; } = new();
}

public sealed class CommandResultData
{
    [JsonPropertyName("commandId")]
    public string CommandId { get; init; } = string.Empty;

    [JsonPropertyName("accepted")]
    public bool Accepted { get; init; }

    [JsonPropertyName("revision")]
    public long Revision { get; init; }

    [JsonPropertyName("error")]
    public ProtocolError? Error { get; init; }
}

public sealed class ProtocolError
{
    [JsonPropertyName("code")]
    public string Code { get; init; } = string.Empty;

    [JsonPropertyName("message")]
    public string Message { get; init; } = string.Empty;
}

public sealed class MatchView
{
    [JsonPropertyName("matchId")]
    public string MatchId { get; init; } = string.Empty;

    [JsonPropertyName("revision")]
    public long Revision { get; init; }

    [JsonPropertyName("lastEventSeq")]
    public long LastEventSeq { get; init; }

    [JsonPropertyName("turn")]
    public int Turn { get; init; }

    [JsonPropertyName("activeSeat")]
    public int ActiveSeat { get; init; }

    [JsonPropertyName("status")]
    public string Status { get; init; } = string.Empty;

    [JsonPropertyName("winnerSeat")]
    public int? WinnerSeat { get; init; }

    [JsonPropertyName("youSeat")]
    public int YouSeat { get; init; }

    [JsonPropertyName("players")]
    public List<PlayerView> Players { get; init; } = [];
}

public sealed class PlayerView
{
    [JsonPropertyName("seat")]
    public int Seat { get; init; }

    [JsonPropertyName("heroId")]
    public string HeroId { get; init; } = string.Empty;

    [JsonPropertyName("hp")]
    public int Hp { get; init; }

    [JsonPropertyName("energy")]
    public int Energy { get; init; }

    [JsonPropertyName("turnNumber")]
    public int TurnNumber { get; init; }

    [JsonPropertyName("deckCount")]
    public int DeckCount { get; init; }

    [JsonPropertyName("handCount")]
    public int HandCount { get; init; }

    [JsonPropertyName("hand")]
    public List<CardView> Hand { get; init; } = [];

    [JsonPropertyName("board")]
    public List<UnitView> Board { get; init; } = [];
}

public sealed class CardView
{
    [JsonPropertyName("instanceId")]
    public string InstanceId { get; init; } = string.Empty;

    [JsonPropertyName("cardId")]
    public string CardId { get; init; } = string.Empty;

    [JsonPropertyName("name")]
    public string Name { get; init; } = string.Empty;

    [JsonPropertyName("cost")]
    public int Cost { get; init; }

    [JsonPropertyName("kind")]
    public string Kind { get; init; } = string.Empty;
}

public sealed class UnitView
{
    [JsonPropertyName("instanceId")]
    public string InstanceId { get; init; } = string.Empty;

    [JsonPropertyName("cardId")]
    public string CardId { get; init; } = string.Empty;

    [JsonPropertyName("name")]
    public string Name { get; init; } = string.Empty;

    [JsonPropertyName("attack")]
    public int Attack { get; init; }

    [JsonPropertyName("health")]
    public int Health { get; init; }

    [JsonPropertyName("maxHealth")]
    public int MaxHealth { get; init; }

    [JsonPropertyName("guard")]
    public bool Guard { get; init; }

    [JsonPropertyName("attacksRemaining")]
    public int AttacksRemaining { get; init; }
}

public sealed class GameEvent
{
    [JsonPropertyName("seq")]
    public long Seq { get; init; }

    [JsonPropertyName("revision")]
    public long Revision { get; init; }

    [JsonPropertyName("type")]
    public string Type { get; init; } = string.Empty;

    [JsonPropertyName("actorSeat")]
    public int ActorSeat { get; init; }

    [JsonPropertyName("visibility")]
    public string Visibility { get; init; } = string.Empty;

    [JsonPropertyName("data")]
    public JsonElement Data { get; init; }
}
