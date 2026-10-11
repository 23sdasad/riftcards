using Godot;
using Riftcards.Client.Core.Protocol;

namespace Riftcards.Client.Godot.Match;

/// <summary>
/// 英雄面板：归属与回合指示、生命/费用/牌库/手牌数量，以及对手英雄的攻击入口。
///
/// 投影不含最大生命（`PlayerView` 只有 `hp`），因此界面不做生命比例着色，只在阵亡（0）时标红；
/// 攻击入口只在“已选攻击者、等待目标”时出现，避免常驻一条横跨整屏的禁用按钮。
/// </summary>
public sealed partial class HeroPanel : PanelContainer
{
    private Label _title = null!;
    private Label _health = null!;
    private Label _stats = null!;
    private Button _targetButton = null!;

    /// <summary>英雄所属座位。</summary>
    public int Seat { get; private set; } = -1;

    /// <summary>点击“攻击此英雄”时触发，参数为座位号。</summary>
    public event Action<int>? TargetPressed;

    public override void _Ready()
    {
        AddThemeStyleboxOverride("panel", UiTheme.PanelStyle());

        // 单行布局：归属与回合指示、生命、其余数值、攻击入口都在一行，纵向更省空间。
        var row = new HBoxContainer { SizeFlagsHorizontal = SizeFlags.ExpandFill };
        row.AddThemeConstantOverride("separation", UiTheme.ElementSeparation);
        AddChild(row);

        _title = UiTheme.Heading("—");
        row.AddChild(_title);

        _health = UiTheme.Body("HP —");
        row.AddChild(_health);

        _stats = UiTheme.StatLine("费用 — │ 牌库 — │ 手牌 —");
        _stats.SizeFlagsHorizontal = SizeFlags.ExpandFill;
        row.AddChild(_stats);

        _targetButton = new Button { Text = "攻击此英雄", Visible = false };
        UiTheme.StyleButton(_targetButton, UiTheme.Danger);
        _targetButton.Pressed += () => TargetPressed?.Invoke(Seat);
        row.AddChild(_targetButton);
    }

    public void Configure(PlayerView player, bool isYou, bool active, bool awaitingTarget, bool finished)
    {
        Seat = player.Seat;
        var who = isYou ? "你" : "对手";
        var turn = active ? (isYou ? " · 你的回合" : " · 对手回合") : string.Empty;
        _title.Text = $"{who}（座位 {player.Seat}）{turn}";
        _title.AddThemeColorOverride("font_color",
            active ? (isYou ? UiTheme.Accent : UiTheme.Warning) : UiTheme.TextPrimary);

        _health.Text = $"HP {player.Hp}";
        _health.AddThemeColorOverride("font_color", player.Hp <= 0 ? UiTheme.Danger : UiTheme.TextPrimary);

        _stats.Text = $"费用 {player.Energy} │ 牌库 {player.DeckCount} │ 手牌 {player.HandCount}";

        _targetButton.Visible = !isYou;
        _targetButton.Disabled = !awaitingTarget || finished;
    }

    /// <summary>没有对局时显示占位内容。</summary>
    public void ConfigureEmpty(string who)
    {
        Seat = -1;
        _title.Text = who;
        _title.AddThemeColorOverride("font_color", UiTheme.TextPrimary);
        _health.Text = "HP —";
        _health.AddThemeColorOverride("font_color", UiTheme.TextMuted);
        _stats.Text = "费用 — │ 牌库 — │ 手牌 —";
        _targetButton.Visible = false;
    }
}
