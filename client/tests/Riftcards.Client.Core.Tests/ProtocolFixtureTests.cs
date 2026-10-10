using System.Text.Json;
using System.Text.Json.Serialization;
using Riftcards.Client.Core.Protocol;
using Xunit;

namespace Riftcards.Client.Core.Tests;

/// <summary>
/// 读取 ai-docs/contracts/protocol-fixtures 下的共享样例，验证 C# 端能解析出等价 DTO。
/// 同一组样例由 Go 侧的 server/internal/protocol/fixtures_test.go 读取，两端断言必须一致。
/// </summary>
public sealed class ProtocolFixtureTests
{
    [Fact]
    public void ParsesEveryServerFixtureIntoEquivalentDto()
    {
        var samples = LoadSamples("server-messages.json");
        Assert.NotEmpty(samples);

        foreach (var sample in samples)
        {
            var envelope = ProtocolJson.Deserialize<ServerEnvelope>(sample.Message.GetRawText());
            var value = Assert.IsType<ServerEnvelope>(envelope);

            var isBroadcast = value.Type is ProtocolNames.MatchStarted or ProtocolNames.MatchEvents;
            if (isBroadcast)
            {
                Assert.Null(value.RequestId);
            }
            else
            {
                Assert.False(string.IsNullOrEmpty(value.RequestId), $"{sample.Name} 是响应，必须带 requestId");
            }

            switch (value.Type)
            {
                case ProtocolNames.Welcome:
                    var welcome = Assert.IsType<WelcomeData>(value.Data.Deserialize<WelcomeData>(ProtocolJson.Options));
                    Assert.Equal("c_4a91", welcome.ConnectionId);
                    Assert.Equal("p_01f2", welcome.PlayerId);
                    Assert.Equal("Player One", welcome.DisplayName);
                    Assert.Equal("0.1.0", welcome.ServerVersion);
                    Assert.Equal(ProtocolNames.Version, welcome.ProtocolVersion);
                    break;

                case ProtocolNames.QueueStatus:
                    var queue = Assert.IsType<QueueStatusData>(value.Data.Deserialize<QueueStatusData>(ProtocolJson.Options));
                    Assert.Contains(queue.State, new[] { "waiting", "left" });
                    break;

                case ProtocolNames.MatchStarted:
                    var started = Assert.IsType<MatchStartedData>(value.Data.Deserialize<MatchStartedData>(ProtocolJson.Options));
                    Assert.Equal("m_9a2f", started.MatchId);
                    Assert.Equal(0, started.Seat);
                    Assert.Equal(2, started.State.Players.Count);
                    Assert.Single(started.State.Players[0].Hand);
                    Assert.Equal("ember_squire", started.State.Players[0].Hand[0].CardId);
                    Assert.Empty(started.State.Players[1].Hand);
                    Assert.Equal(4, started.State.Players[1].HandCount);
                    break;

                case ProtocolNames.MatchCommandResult:
                    var result = Assert.IsType<CommandResultData>(value.Data.Deserialize<CommandResultData>(ProtocolJson.Options));
                    Assert.False(string.IsNullOrEmpty(result.CommandId));
                    if (result.Accepted)
                    {
                        Assert.Null(result.Error);
                    }
                    else
                    {
                        Assert.NotNull(result.Error);
                        Assert.False(string.IsNullOrEmpty(result.Error!.Code));
                    }
                    break;

                case ProtocolNames.MatchEvents:
                    var events = Assert.IsType<MatchEventsData>(value.Data.Deserialize<MatchEventsData>(ProtocolJson.Options));
                    Assert.Equal("m_9a2f", events.MatchId);
                    Assert.Equal(0, events.BaseRevision);
                    Assert.Equal(1, events.Revision);
                    Assert.Equal(2, events.Events.Count);
                    Assert.Equal(1, events.Events[0].Seq);
                    Assert.Equal("unit_summoned", events.Events[1].Type);
                    Assert.Equal(events.Revision, events.State.Revision);
                    break;

                case ProtocolNames.Error:
                    var error = Assert.IsType<ProtocolError>(value.Data.Deserialize<ProtocolError>(ProtocolJson.Options));
                    Assert.False(string.IsNullOrEmpty(error.Code));
                    Assert.False(string.IsNullOrEmpty(error.Message));
                    break;

                case ProtocolNames.Pong:
                    var pong = Assert.IsType<PongData>(value.Data.Deserialize<PongData>(ProtocolJson.Options));
                    Assert.False(string.IsNullOrEmpty(pong.ServerTime));
                    break;

                default:
                    Assert.Fail($"样例包含未覆盖的服务端消息类型 {value.Type}");
                    break;
            }
        }
    }

    [Fact]
    public void ParsesEveryClientFixtureIntoEquivalentDto()
    {
        var samples = LoadSamples("client-messages.json");
        var seen = new HashSet<string>();

        foreach (var sample in samples)
        {
            var view = Assert.IsType<FixtureEnvelope>(
                ProtocolJson.Deserialize<FixtureEnvelope>(sample.Message.GetRawText()));
            Assert.False(string.IsNullOrEmpty(view.RequestId), $"{sample.Name} 必须带 requestId");
            seen.Add(view.Type);

            // 真实发送用的 DTO 也必须能解析同一份样例。
            var clientEnvelope = Assert.IsType<ClientEnvelope>(
                ProtocolJson.Deserialize<ClientEnvelope>(sample.Message.GetRawText()));
            Assert.Equal(view.Type, clientEnvelope.Type);
            Assert.Equal(view.RequestId, clientEnvelope.RequestId);

            switch (view.Type)
            {
                case ProtocolNames.Hello:
                    var hello = Assert.IsType<HelloData>(view.Data.Deserialize<HelloData>(ProtocolJson.Options));
                    Assert.Equal("Player One", hello.DisplayName);
                    Assert.Equal(ProtocolNames.Version, hello.ProtocolVersion);
                    break;

                case ProtocolNames.QueueJoin:
                case ProtocolNames.QueueLeave:
                case ProtocolNames.Ping:
                    Assert.Empty(view.Data.EnumerateObject());
                    break;

                case ProtocolNames.MatchCommand:
                    var command = Assert.IsType<MatchCommandRequest>(view.Data.Deserialize<MatchCommandRequest>(ProtocolJson.Options));
                    Assert.Equal("m_9a2f", command.MatchId);
                    Assert.False(string.IsNullOrEmpty(command.CommandId));
                    Assert.False(string.IsNullOrEmpty(command.Command.Type));
                    break;

                default:
                    Assert.Fail($"样例包含未覆盖的客户端消息类型 {view.Type}");
                    break;
            }
        }

        foreach (var messageType in new[]
                 {
                     ProtocolNames.Hello, ProtocolNames.QueueJoin, ProtocolNames.QueueLeave,
                     ProtocolNames.MatchCommand, ProtocolNames.Ping,
                 })
        {
            Assert.Contains(messageType, seen);
        }
    }

    [Fact]
    public void GeneratedCommandMatchesFixture()
    {
        var sample = LoadSamples("client-messages.json")
            .Single(entry => entry.Name == "match.command.play_card");
        var view = Assert.IsType<FixtureEnvelope>(
            ProtocolJson.Deserialize<FixtureEnvelope>(sample.Message.GetRawText()));
        var fromFixture = Assert.IsType<MatchCommandRequest>(
            view.Data.Deserialize<MatchCommandRequest>(ProtocolJson.Options));

        var generated = new MatchCommandRequest
        {
            MatchId = "m_9a2f",
            CommandId = "cmd-001",
            ExpectedRevision = 0,
            Command = new PlayerCommand
            {
                Type = ProtocolNames.PlayCard,
                CardInstanceId = "p0-c0",
            },
        };

        Assert.Equal(fromFixture.MatchId, generated.MatchId);
        Assert.Equal(fromFixture.CommandId, generated.CommandId);
        Assert.Equal(fromFixture.ExpectedRevision, generated.ExpectedRevision);
        Assert.Equal(fromFixture.Command.Type, generated.Command.Type);
        Assert.Equal(fromFixture.Command.CardInstanceId, generated.Command.CardInstanceId);

        // 生成方向：序列化后再解析必须等价。
        var roundTrip = Assert.IsType<FixtureEnvelope>(ProtocolJson.Deserialize<FixtureEnvelope>(
            ProtocolJson.Serialize(new ClientEnvelope
            {
                Type = ProtocolNames.MatchCommand,
                RequestId = "req-command",
                Data = generated,
            })));
        var reparsed = Assert.IsType<MatchCommandRequest>(
            roundTrip.Data.Deserialize<MatchCommandRequest>(ProtocolJson.Options));
        Assert.Equal(generated.CommandId, reparsed.CommandId);
        Assert.Equal(generated.Command.CardInstanceId, reparsed.Command.CardInstanceId);
    }

    private static List<FixtureSample> LoadSamples(string fileName)
    {
        var path = Path.Combine(FixtureDirectory(), fileName);
        var file = JsonSerializer.Deserialize<FixtureFile>(File.ReadAllText(path), ProtocolJson.Options);
        var value = Assert.IsType<FixtureFile>(file);
        Assert.NotEmpty(value.Messages);
        return value.Messages;
    }

    /// <summary>从测试程序集位置向上查找共享样例目录，不依赖当前工作目录。</summary>
    private static string FixtureDirectory()
    {
        var directory = new DirectoryInfo(AppContext.BaseDirectory);
        while (directory is not null)
        {
            var candidate = Path.Combine(directory.FullName, "ai-docs", "contracts", "protocol-fixtures");
            if (Directory.Exists(candidate))
            {
                return candidate;
            }
            directory = directory.Parent;
        }
        throw new InvalidOperationException("未找到 ai-docs/contracts/protocol-fixtures");
    }

    private sealed class FixtureFile
    {
        public string Version { get; init; } = string.Empty;

        public List<FixtureSample> Messages { get; init; } = [];
    }

    private sealed class FixtureSample
    {
        public string Name { get; init; } = string.Empty;

        public JsonElement Message { get; init; }
    }

    /// <summary>样例信封视图：载荷保留为 JsonElement，便于按消息类型断言。</summary>
    private sealed class FixtureEnvelope
    {
        [JsonPropertyName("type")]
        public string Type { get; init; } = string.Empty;

        [JsonPropertyName("requestId")]
        public string? RequestId { get; init; }

        [JsonPropertyName("data")]
        public JsonElement Data { get; init; }
    }
}
