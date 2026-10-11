using Godot;
using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;
using Riftcards.Client.Godot;
using Riftcards.Client.Godot.Match;

namespace Riftcards.Client;

/// <summary>
/// 应用壳：顶栏（连接设置与会话状态）、对局面板与事件面板。
/// 对局交互在 <see cref="MatchBoardView"/>，事件播放由 <see cref="EventLogPanel"/> 消费队列，
/// 这里不做任何规则判断。视觉约定见 ai-docs/architecture/client-ui.md。
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
    private string? _screenshotPath;
    private ulong _screenshotDeadlineMs;

    public override void _Ready()
    {
        RenderingServer.SetDefaultClearColor(UiTheme.Background);

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
        margin.AddThemeConstantOverride("margin_left", 16);
        margin.AddThemeConstantOverride("margin_top", 12);
        margin.AddThemeConstantOverride("margin_right", 16);
        margin.AddThemeConstantOverride("margin_bottom", 12);
        AddChild(margin);

        var root = new VBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        root.AddThemeConstantOverride("separation", UiTheme.SectionSeparation);
        margin.AddChild(root);

        // 顶栏：标题、会话状态、服务器地址与连接操作。
        var topBar = new HBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        topBar.AddThemeConstantOverride("separation", UiTheme.ElementSeparation);
        topBar.AddChild(UiTheme.Title("riftcards"));

        _statusLabel = UiTheme.Caption("未连接", UiTheme.TextSecondary);
        _statusLabel.SizeFlagsHorizontal = SizeFlags.ExpandFill;
        _statusLabel.VerticalAlignment = VerticalAlignment.Center;
        topBar.AddChild(_statusLabel);

        _serverInput = new LineEdit
        {
            Text = "ws://127.0.0.1:8080/ws",
            SizeFlagsHorizontal = SizeFlags.ExpandFill,
            CustomMinimumSize = new Vector2(260, 0),
        };
        topBar.AddChild(_serverInput);

        var connectButton = new Button { Text = "连接" };
        UiTheme.StyleButton(connectButton, UiTheme.Accent);
        connectButton.Pressed += Connect;
        topBar.AddChild(connectButton);

        var queueButton = new Button { Text = "进入匹配" };
        UiTheme.StyleButton(queueButton, UiTheme.Accent);
        queueButton.Pressed += _session.JoinQueue;
        topBar.AddChild(queueButton);

        root.AddChild(UiTheme.PanelBox(topBar));

        // 对局区优先占用剩余空间；事件面板固定小高度并位于底部。
        _boardView = new MatchBoardView
        {
            SizeFlagsHorizontal = SizeFlags.ExpandFill,
            SizeFlagsVertical = SizeFlags.ExpandFill,
        };
        root.AddChild(_boardView);

        _eventPanel = new EventLogPanel { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        root.AddChild(_eventPanel);
    }

    private void Connect()
    {
        _session.Connect(_serverInput.Text.Trim(), $"Player-{GetInstanceId()}");
    }

    /// <summary>
    /// 开发用自动连接：`godot --path client -- --server=ws://127.0.0.1:8080/ws`。
    /// 连接成功后 <see cref="GameSession"/> 会自动发送 hello 并进入匹配，因此无需额外操作。
    /// </summary>
    private void TryAutoStart()
    {
        // 截图与自动出牌不依赖 --server=，但通常一起使用。
        _screenshotPath = ReadUserArg("--screenshot=");
        if (_screenshotPath is not null)
        {
            var delay = ReadUserArg("--screenshot-delay=");
            var seconds = double.TryParse(delay, out var parsed) ? parsed : 8.0;
            // 按真实时间计时，不按帧数：窗口未限帧时帧数会飞快跑完。
            _screenshotDeadlineMs = Time.GetTicksMsec() + (ulong)Math.Max(100, seconds * 1000);
            GD.Print($"[riftcards] screenshot {_screenshotPath} after {seconds:0.#}s");
        }
        if (ReadUserArg("--smoke-autoplay") is not null)
        {
            _boardView.EnableAutoPlay();
            GD.Print("[riftcards] smoke: autoplay enabled");
        }

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

    /// <summary>开发用截图：等待指定时间后保存视口并退出，用于人工核对界面布局。</summary>
    public override void _Process(double delta)
    {
        if (_screenshotPath is null || Time.GetTicksMsec() < _screenshotDeadlineMs)
        {
            return;
        }

        var path = _screenshotPath;
        _screenshotPath = null;
        var image = GetViewport().GetTexture().GetImage();
        var error = image.SavePng(path);
        GD.Print($"[riftcards] screenshot saved={path} size={image.GetWidth()}x{image.GetHeight()} error={error}");
        GetTree().Quit();
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
