namespace Riftcards.Client.Core.Session;

public interface IMessageTransport
{
    event Action? Connected;
    event Action<string?>? Disconnected;
    event Action<string>? TextReceived;

    bool IsOpen { get; }

    void Connect(string url);

    void SendText(string text);
}
