using System.Text;
using Godot;
using Riftcards.Client.Core.Session;

namespace Riftcards.Client.Godot;

public sealed partial class GodotWebSocketTransport : Node, IMessageTransport
{
    private readonly Queue<string> _pending = new();
    private WebSocketPeer? _peer;
    private bool _wasOpen;

    public event Action? Connected;

    public event Action<string?>? Disconnected;

    public event Action<string>? TextReceived;

    public bool IsOpen => _peer?.GetReadyState() == WebSocketPeer.State.Open;

    public void Connect(string url)
    {
        _pending.Clear();
        _peer = new WebSocketPeer();
        var error = _peer.ConnectToUrl(url);
        if (error != Error.Ok)
        {
            Disconnected?.Invoke($"connect failed: {error}");
        }
    }

    public void SendText(string text)
    {
        if (IsOpen)
        {
            Flush(text);
            return;
        }
        _pending.Enqueue(text);
    }

    public override void _Process(double delta)
    {
        if (_peer is null)
        {
            return;
        }

        _peer.Poll();
        var state = _peer.GetReadyState();
        if (state == WebSocketPeer.State.Open && !_wasOpen)
        {
            _wasOpen = true;
            Connected?.Invoke();
            while (_pending.Count > 0)
            {
                Flush(_pending.Dequeue());
            }
        }
        else if (state == WebSocketPeer.State.Closed && _wasOpen)
        {
            _wasOpen = false;
            Disconnected?.Invoke(_peer.GetCloseReason());
        }

        while (_peer.GetAvailablePacketCount() > 0)
        {
            var packet = _peer.GetPacket();
            TextReceived?.Invoke(Encoding.UTF8.GetString(packet));
        }
    }

    public override void _ExitTree()
    {
        _peer?.Close();
        _peer = null;
    }

    private void Flush(string text)
    {
        var error = _peer?.SendText(text) ?? Error.Unavailable;
        if (error != Error.Ok)
        {
            Disconnected?.Invoke($"send failed: {error}");
        }
    }
}
