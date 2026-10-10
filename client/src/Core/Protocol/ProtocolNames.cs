namespace Riftcards.Client.Core.Protocol;

public static class ProtocolNames
{
    public const string Version = "0.1";

    public const string Hello = "hello";
    public const string QueueJoin = "queue.join";
    public const string QueueLeave = "queue.leave";
    public const string MatchCommand = "match.command";
    public const string Ping = "ping";

    public const string Welcome = "welcome";
    public const string QueueStatus = "queue.status";
    public const string MatchStarted = "match.started";
    public const string MatchCommandResult = "match.command_result";
    public const string MatchEvents = "match.events";
    public const string Error = "error";
    public const string Pong = "pong";

    public const string PlayCard = "play_card";
    public const string Attack = "attack";
    public const string EndTurn = "end_turn";
    public const string Surrender = "surrender";

    public const string CardKindUnit = "unit";
    public const string CardKindSpell = "spell";

    /// <summary>
    /// 已知领域事件类型（见 ai-docs/contracts/protocol.md）。未知类型必须被记录并忽略，
    /// 不得改变客户端状态。
    /// </summary>
    public static readonly IReadOnlySet<string> KnownEventTypes = new HashSet<string>(StringComparer.Ordinal)
    {
        "turn_started",
        "turn_ended",
        "card_drawn",
        "card_burned",
        "card_played",
        "unit_summoned",
        "attack_resolved",
        "damage_dealt",
        "healed",
        "unit_died",
        "fatigue_damage",
        "match_ended",
    };
}
