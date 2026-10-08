namespace Lscs.Client.Core.Protocol;

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
}
