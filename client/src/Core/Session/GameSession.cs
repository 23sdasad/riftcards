using System.Text.Json;
using Riftcards.Client.Core.Protocol;

namespace Riftcards.Client.Core.Session;

public sealed class GameSession
{
    private readonly IMessageTransport _transport;
    private readonly List<GameEvent> _events = [];
    private readonly Dictionary<string, PendingRequest> _pending = [];
    private int _requestCounter;
    private string _displayName = "Player";

    public GameSession(IMessageTransport transport)
    {
        _transport = transport;
        _transport.Connected += HandleConnected;
        _transport.Disconnected += HandleDisconnected;
        _transport.TextReceived += HandleTextReceived;
    }

    public event Action? Changed;

    public string Status { get; private set; } = "disconnected";

    public MatchView? View { get; private set; }

    public IReadOnlyList<GameEvent> Events => _events;

    public bool IsConnected => _transport.IsOpen;

    /// <summary>最近一条带 requestId 的响应所关联的请求 ID；广播不改变该值。</summary>
    public string? LastResponseRequestId { get; private set; }

    /// <summary>最近一次 match.command_result 的结果，含原始请求 ID。</summary>
    public CommandOutcome? LastCommandOutcome { get; private set; }

    /// <summary>最近一次 error 响应，含原始请求 ID。</summary>
    public ProtocolError? LastError { get; private set; }

    /// <summary>最近一次 error 响应关联的请求 ID。</summary>
    public string? LastErrorRequestId { get; private set; }

    /// <summary>尚未收到响应的请求数量。</summary>
    public int PendingRequests => _pending.Count;

    public void Connect(string url, string displayName)
    {
        _displayName = string.IsNullOrWhiteSpace(displayName) ? "Player" : displayName;
        Status = "connecting";
        Changed?.Invoke();
        _transport.Connect(url);
    }

    public void Queue()
    {
        Send(ProtocolNames.QueueJoin, new { });
    }

    public void LeaveQueue()
    {
        Send(ProtocolNames.QueueLeave, new { });
    }

    public void Ping()
    {
        Send(ProtocolNames.Ping, new { });
    }

    public void SubmitCommand(string type, string? cardInstanceId = null, string? targetId = null)
    {
        if (View is null)
        {
            Status = "match is not ready";
            Changed?.Invoke();
            return;
        }

        var commandId = NextRequestId("cmd");
        var request = new MatchCommandRequest
        {
            MatchId = View.MatchId,
            CommandId = commandId,
            ExpectedRevision = View.Revision,
            Command = new PlayerCommand
            {
                Type = type,
                CardInstanceId = cardInstanceId,
                TargetId = targetId,
            },
        };
        Send(ProtocolNames.MatchCommand, request, commandId);
    }

    private void HandleConnected()
    {
        Status = "connected";
        Send(ProtocolNames.Hello, new HelloData { DisplayName = _displayName });
        Queue();
        Changed?.Invoke();
    }

    private void HandleDisconnected(string? reason)
    {
        Status = string.IsNullOrWhiteSpace(reason) ? "disconnected" : $"disconnected: {reason}";
        Changed?.Invoke();
    }

    private void HandleTextReceived(string text)
    {
        ServerEnvelope? envelope;
        try
        {
            envelope = ProtocolJson.Deserialize<ServerEnvelope>(text);
        }
        catch (JsonException exception)
        {
            Status = $"protocol error: {exception.Message}";
            Changed?.Invoke();
            return;
        }

        if (envelope is null)
        {
            return;
        }

        // 带 requestId 的消息是响应：记录它，并把对应的待处理请求标记为已完成。
        if (!string.IsNullOrEmpty(envelope.RequestId))
        {
            LastResponseRequestId = envelope.RequestId;
            _pending.Remove(envelope.RequestId);
        }

        switch (envelope.Type)
        {
            case ProtocolNames.Welcome:
                var welcome = envelope.Data.Deserialize<WelcomeData>(ProtocolJson.Options);
                Status = welcome is null ? "welcome" : $"welcome {welcome.DisplayName}";
                break;

            case ProtocolNames.QueueStatus:
                var queue = envelope.Data.Deserialize<QueueStatusData>(ProtocolJson.Options);
                if (queue is null)
                {
                    Status = "queue";
                    break;
                }
                Status = queue.State switch
                {
                    "waiting" => $"queue: waiting ({queue.Position})",
                    "left" => "queue: left",
                    _ => "queue",
                };
                break;

            case ProtocolNames.MatchStarted:
                var started = envelope.Data.Deserialize<MatchStartedData>(ProtocolJson.Options);
                if (started is not null)
                {
                    View = started.State;
                    Status = $"match {started.MatchId}, seat {started.Seat}";
                }
                break;

            case ProtocolNames.MatchCommandResult:
                var result = envelope.Data.Deserialize<CommandResultData>(ProtocolJson.Options);
                if (result is not null)
                {
                    LastCommandOutcome = new CommandOutcome(
                        envelope.RequestId ?? string.Empty,
                        result.CommandId,
                        result.Accepted,
                        result.Revision,
                        result.Error?.Code);
                    Status = result.Accepted
                        ? $"command accepted, revision {result.Revision}"
                        : $"command rejected: {result.Error?.Code ?? "unknown"}";
                }
                break;

            case ProtocolNames.MatchEvents:
                var events = envelope.Data.Deserialize<MatchEventsData>(ProtocolJson.Options);
                if (events is not null)
                {
                    _events.AddRange(events.Events);
                    View = events.State;
                }
                break;

            case ProtocolNames.Error:
                var error = envelope.Data.Deserialize<ProtocolError>(ProtocolJson.Options);
                LastError = error;
                LastErrorRequestId = envelope.RequestId;
                Status = $"server error: {error?.Code ?? "unknown"}";
                break;

            case ProtocolNames.Pong:
                Status = "pong";
                break;

            default:
                Status = $"unknown message: {envelope.Type}";
                break;
        }

        Changed?.Invoke();
    }

    private string Send(string type, object data, string? commandId = null)
    {
        var requestId = NextRequestId("req");
        var envelope = new ClientEnvelope
        {
            Type = type,
            RequestId = requestId,
            Data = data,
        };
        _pending[requestId] = new PendingRequest(type, commandId);
        _transport.SendText(ProtocolJson.Serialize(envelope));
        return requestId;
    }

    private string NextRequestId(string prefix)
    {
        var value = Interlocked.Increment(ref _requestCounter);
        return $"{prefix}-{value}";
    }

    /// <summary>已发出但尚未收到响应的请求。</summary>
    private sealed record PendingRequest(string Kind, string? CommandId);
}

/// <summary>一次 match.command 的结果，保留关联的请求 ID。</summary>
public sealed record CommandOutcome(
    string RequestId,
    string CommandId,
    bool Accepted,
    long Revision,
    string? ErrorCode);
