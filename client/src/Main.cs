using Godot;
using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;
using Riftcards.Client.Godot;
using Riftcards.Client.Godot.Match;

namespace Riftcards.Client;

/// <summary>
/// 应用壳：连接设置、会话状态与事件面板。对局交互在 <see cref="MatchBoardView"/>，
/// 事件播放由 <see cref="EventLogPanel"/> 消费队列，这里不做任何规则判断。
/// </summary>
public sealed partial class Main : Control
{
    private GodotWebSocketTransport _transport = null!;
    private GameSession _session = null!;
    private LineEdit _serverInput = null!;
    private Label _statusLabel = null!;
    private MatchBoardView _boardView = null!;
    private EventLogPanel _eventPanel = null!;
    private string _lastLoggedStatus = string.Empty;
    private bool _smokeEndTurn;
    private bool _smokeCommandSent;

    public override void _Ready()
    {
        _transport = new GodotWebSocketTransport();
        AddChild(_transport);
        _session = new GameSession(_transport);

        BuildUi();

        _session.Changed += Refresh;
        _boardView.Bind(_session);
        _eventPanel.Bind(_session);
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
        queueButton.Pressed += _session.JoinQueue;
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

        _eventPanel = new EventLogPanel();
        root.AddChild(_eventPanel);
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
        _smokeEndTurn = ReadUserArg("--smoke-end-turn") is not null;
        GD.Print($"[riftcards] autostart {server} (smoke-end-turn={_smokeEndTurn})");
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
        SubmitSmokeCommand();
    }

    /// <summary>
    /// 开发用冒烟动作：`--smoke-end-turn` 时，在自己回合提交一次结束回合，
    /// 让无界面冒烟也能观察到真实事件顺序。
    /// </summary>
    private void SubmitSmokeCommand()
    {
        if (!_smokeEndTurn || _smokeCommandSent)
        {
            return;
        }
        if (_session.View is not { } view || view.Status != "active" || view.ActiveSeat != view.YouSeat)
        {
            return;
        }
        _smokeCommandSent = true;
        GD.Print("[riftcards] smoke: submitting end_turn");
        _session.SubmitCommand(ProtocolNames.EndTurn);
    }
}
