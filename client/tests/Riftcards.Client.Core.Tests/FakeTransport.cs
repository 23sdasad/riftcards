using Riftcards.Client.Core.Session;

namespace Riftcards.Client.Core.Tests;

/// <summary>测试用传输：记录发出的文本，并允许注入服务端消息。</summary>
internal sealed class FakeTransport : IMessageTransport
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

    public void Disconnect(string? reason = null)
    {
        IsOpen = false;
        Disconnected?.Invoke(reason);
    }
}
