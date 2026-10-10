using System.Linq;
using Godot;
using Riftcards.Client.Core.Session;
using Riftcards.Client.Godot;
using Riftcards.Client.Godot.Match;

namespace Riftcards.Client;

/// <summary>
/// 应用壳：连接设置、会话状态与事件日志。对局交互全部在 <see cref="MatchBoardView"/>，
/// 这里不做任何规则或目标选择判断。
/// </summary>
public sealed partial class Main : Control
{
    private GodotWebSocketTransport _transport = null!;
    private GameSession _session = null!;
    private LineEdit _serverInput = null!;
    private Label _statusLabel = null!;
    private MatchBoardView _boardView = null!;
    private RichTextLabel _logLabel = null!;
    private string _lastLoggedStatus = string.Empty;

    public override void _Ready()
    {
        _transport = new GodotWebSocketTransport();
        AddChild(_transport);
        _session = new GameSession(_transport);

        BuildUi();

        _session.Changed += Refresh;
        _boardView.Bind(_session);
        Refresh();
        TryAutoStart();
    }

    private void BuildUi()
    {
        SetAnchorsAndOffsetsPreset(LayoutPreset.FullRect);

        var margin = new MarginContainer();
        margin.SetAnchorsAndOffsetsPreset(LayoutPreset.FullRect);
        margin.AddThemeConstantOverride("margin_left", 20);
        margin.AddThemeConstantOverride("margin_top", 16);
        margin.AddThemeConstantOverride("margin_right", 20);
        margin.AddThemeConstantOverride("margin_bottom", 16);
        AddChild(margin);

        var root = new VBoxContainer();
        root.AddThemeConstantOverride("separation", 8);
        margin.AddChild(root);

        root.AddChild(new Label { Text = "riftcards" });

        var serverRow = new HBoxContainer();
        serverRow.AddThemeConstantOverride("separation", 8);
        root.AddChild(serverRow);

        _serverInput = new LineEdit
        {
            Text = "ws://127.0.0.1:8080/ws",
            SizeFlagsHorizontal = SizeFlags.ExpandFill,
        };
        serverRow.AddChild(_serverInput);

        var connectButton = new Button { Text = "连接" };
        connectButton.Pressed += Connect;
        serverRow.AddChild(connectButton);

        var queueButton = new Button { Text = "进入匹配" };
        queueButton.Pressed += _session.Queue;
        serverRow.AddChild(queueButton);

        _statusLabel = new Label { AutowrapMode = TextServer.AutowrapMode.WordSmart };
        root.AddChild(_statusLabel);

        // 用滚动容器包住对局面板：窗口变小时核心操作仍然可达。
        var scroll = new ScrollContainer
        {
            SizeFlagsHorizontal = SizeFlags.ExpandFill,
            SizeFlagsVertical = SizeFlags.ExpandFill,
        };
        root.AddChild(scroll);

        _boardView = new MatchBoardView { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        scroll.AddChild(_boardView);

        _logLabel = new RichTextLabel
        {
            FitContent = false,
            ScrollActive = true,
            CustomMinimumSize = new Vector2(0, 160),
        };
        root.AddChild(_logLabel);
    }

    private void Connect()
    {
        _session.Connect(_serverInput.Text.Trim(), $"Player-{GetInstanceId()}");
    }

    /// <summary>
    /// 开发用自动连接：`godot --path client -- --server=ws://127.0.0.1:8080/ws`。
    /// 连接成功后 <see cref="GameSession"/> 会自动发送 hello 并进入匹配，因此无需额外操作。
    /// 用于无显示器的本地冒烟。
    /// </summary>
    private void TryAutoStart()
    {
        var server = ReadUserArg("--server=");
        if (string.IsNullOrWhiteSpace(server))
        {
            return;
        }
        _serverInput.Text = server;
        GD.Print($"[riftcards] autostart {server}");
        Connect();
    }

    private static string? ReadUserArg(string prefix)
    {
        foreach (var argument in OS.GetCmdlineUserArgs())
        {
            if (argument.StartsWith(prefix, StringComparison.Ordinal))
            {
                return argument[prefix.Length..];
            }
        }
        return null;
    }

    private void Refresh()
    {
        _statusLabel.Text = _session.Status;
        if (_session.Status != _lastLoggedStatus)
        {
            _lastLoggedStatus = _session.Status;
            GD.Print($"[riftcards] status: {_session.Status}");
        }
        RefreshLog();
    }

    private void RefreshLog()
    {
        var lines = _session.Events
            .TakeLast(60)
            .Select(gameEvent => $"#{gameEvent.Seq} {gameEvent.Type} {gameEvent.Data}");
        _logLabel.Text = string.Join("\n", lines);
    }
}
