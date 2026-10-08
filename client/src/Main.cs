using System.Linq;
using Godot;
using Lscs.Client.Core.Protocol;
using Lscs.Client.Core.Session;
using Lscs.Client.Godot;

namespace Lscs.Client;

public sealed partial class Main : Control
{
    private readonly List<Button> _handButtons = [];
    private readonly List<Button> _boardButtons = [];

    private GodotWebSocketTransport _transport = null!;
    private GameSession _session = null!;
    private LineEdit _serverInput = null!;
    private Label _statusLabel = null!;
    private Label _matchLabel = null!;
    private Label _handLabel = null!;
    private Label _boardLabel = null!;
    private RichTextLabel _logLabel = null!;
    private HBoxContainer _handActions = null!;
    private HBoxContainer _boardActions = null!;
    private Button _endTurnButton = null!;
    private Button _surrenderButton = null!;

    public override void _Ready()
    {
        BuildUi();
        _transport = new GodotWebSocketTransport();
        AddChild(_transport);
        _session = new GameSession(_transport);
        _session.Changed += Refresh;
        Refresh();
    }

    private void BuildUi()
    {
        SetAnchorsAndOffsetsPreset(LayoutPreset.FullRect);

        var margin = new MarginContainer();
        margin.SetAnchorsAndOffsetsPreset(LayoutPreset.FullRect);
        margin.AddThemeConstantOverride("margin_left", 24);
        margin.AddThemeConstantOverride("margin_top", 20);
        margin.AddThemeConstantOverride("margin_right", 24);
        margin.AddThemeConstantOverride("margin_bottom", 20);
        AddChild(margin);

        var root = new VBoxContainer();
        root.AddThemeConstantOverride("separation", 8);
        margin.AddChild(root);

        var title = new Label
        {
            Text = "LSCS",
        };
        root.AddChild(title);

        var serverRow = new HBoxContainer();
        serverRow.AddThemeConstantOverride("separation", 8);
        root.AddChild(serverRow);

        _serverInput = new LineEdit
        {
            Text = "ws://127.0.0.1:8080/ws",
            SizeFlagsHorizontal = SizeFlags.ExpandFill,
        };
        serverRow.AddChild(_serverInput);

        var connectButton = new Button
        {
            Text = "Connect",
        };
        connectButton.Pressed += Connect;
        serverRow.AddChild(connectButton);

        var queueButton = new Button
        {
            Text = "Queue",
        };
        queueButton.Pressed += _session.Queue;
        serverRow.AddChild(queueButton);

        _statusLabel = new Label
        {
            AutowrapMode = TextServer.AutowrapMode.WordSmart,
        };
        root.AddChild(_statusLabel);

        var actionRow = new HBoxContainer();
        actionRow.AddThemeConstantOverride("separation", 8);
        root.AddChild(actionRow);

        _endTurnButton = new Button
        {
            Text = "End Turn",
        };
        _endTurnButton.Pressed += () => _session.SubmitCommand(ProtocolNames.EndTurn);
        actionRow.AddChild(_endTurnButton);

        _surrenderButton = new Button
        {
            Text = "Surrender",
        };
        _surrenderButton.Pressed += () => _session.SubmitCommand(ProtocolNames.Surrender);
        actionRow.AddChild(_surrenderButton);

        _matchLabel = new Label
        {
            AutowrapMode = TextServer.AutowrapMode.WordSmart,
        };
        root.AddChild(_matchLabel);

        _handLabel = new Label();
        root.AddChild(_handLabel);

        _handActions = new HBoxContainer();
        _handActions.AddThemeConstantOverride("separation", 6);
        root.AddChild(_handActions);

        _boardLabel = new Label();
        root.AddChild(_boardLabel);

        _boardActions = new HBoxContainer();
        _boardActions.AddThemeConstantOverride("separation", 6);
        root.AddChild(_boardActions);

        _logLabel = new RichTextLabel
        {
            FitContent = false,
            ScrollActive = true,
            CustomMinimumSize = new Vector2(0, 220),
            SizeFlagsVertical = SizeFlags.ExpandFill,
        };
        root.AddChild(_logLabel);
    }

    private void Connect()
    {
        _session.Connect(_serverInput.Text.Trim(), $"Player-{GetInstanceId()}");
    }

    private void Refresh()
    {
        _statusLabel.Text = _session.Status;
        var view = _session.View;
        if (view is null)
        {
            _matchLabel.Text = "No match";
            _handLabel.Text = "Hand";
            _boardLabel.Text = "Board";
            ClearButtons(_handButtons);
            ClearButtons(_boardButtons);
            _endTurnButton.Disabled = true;
            _surrenderButton.Disabled = true;
            RefreshLog();
            return;
        }

        _endTurnButton.Disabled = view.Status != "active";
        _surrenderButton.Disabled = view.Status != "active";
        _matchLabel.Text = $"Match {view.MatchId} | turn {view.Turn} | active seat {view.ActiveSeat} | revision {view.Revision} | status {view.Status}";

        var me = view.Players.FirstOrDefault(player => player.Seat == view.YouSeat);
        var opponent = view.Players.FirstOrDefault(player => player.Seat != view.YouSeat);
        if (me is null || opponent is null)
        {
            return;
        }

        _handLabel.Text = $"Hand | HP {me.Hp} | energy {me.Energy} | deck {me.DeckCount}";
        _boardLabel.Text = $"Board | enemy HP {opponent.Hp} | enemy hand {opponent.HandCount}";
        RebuildHandButtons(me);
        RebuildBoardButtons(me, opponent);
        RefreshLog();
    }

    private void RebuildHandButtons(PlayerView me)
    {
        ClearButtons(_handButtons);
        foreach (var card in me.Hand)
        {
            var button = new Button
            {
                Text = $"{card.Name} ({card.Cost})",
            };
            button.Pressed += () => PlayCard(card);
            _handButtons.Add(button);
            _handActions.AddChild(button);
        }
    }

    private void RebuildBoardButtons(PlayerView me, PlayerView opponent)
    {
        ClearButtons(_boardButtons);
        foreach (var unit in me.Board)
        {
            if (unit.AttacksRemaining <= 0)
            {
                continue;
            }
            var button = new Button
            {
                Text = $"Attack: {unit.Name}",
            };
            button.Pressed += () => Attack(unit, opponent);
            _boardButtons.Add(button);
            _boardActions.AddChild(button);
        }
    }

    private void PlayCard(CardView card)
    {
        var view = _session.View;
        if (view is null)
        {
            return;
        }

        var targetId = card.CardId switch
        {
            "arcane_bolt" => $"hero-{1 - view.YouSeat}",
            "healing_light" => $"hero-{view.YouSeat}",
            _ => null,
        };
        _session.SubmitCommand(ProtocolNames.PlayCard, card.InstanceId, targetId);
    }

    private void Attack(UnitView unit, PlayerView opponent)
    {
        var guard = opponent.Board.FirstOrDefault(candidate => candidate.Guard);
        var targetId = guard?.InstanceId ?? opponent.HeroId;
        _session.SubmitCommand(ProtocolNames.Attack, unit.InstanceId, targetId);
    }

    private void RefreshLog()
    {
        var lines = _session.Events
            .TakeLast(80)
            .Select(gameEvent => $"#{gameEvent.Seq} {gameEvent.Type} {gameEvent.Data}");
        _logLabel.Text = string.Join("\n", lines);
    }

    private static void ClearButtons(List<Button> buttons)
    {
        foreach (var button in buttons)
        {
            button.QueueFree();
        }
        buttons.Clear();
    }
}
