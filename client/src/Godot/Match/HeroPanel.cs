using Godot;
using Riftcards.Client.Core.Protocol;

namespace Riftcards.Client.Godot.Match;

/// <summary>英雄面板：显示生命、费用、牌库与手牌数量；对手英雄可作为攻击目标。</summary>
public sealed partial class HeroPanel : VBoxContainer
{
    private readonly Label _title = new();
    private readonly Label _stats = new();
    private readonly Button _targetButton = new();

    /// <summary>英雄所属座位。</summary>
    public int Seat { get; private set; }

    /// <summary>点击“攻击此英雄”时触发，参数为座位号。</summary>
    public event Action<int>? TargetPressed;

    public override void _Ready()
    {
        AddThemeConstantOverride("separation", 2);
        AddChild(_title);
        AddChild(_stats);
        _targetButton.Pressed += () => TargetPressed?.Invoke(Seat);
        AddChild(_targetButton);
    }

    public void Configure(PlayerView player, bool isYou, bool awaitingTarget, bool finished)
    {
        Seat = player.Seat;
        _title.Text = isYou ? $"你（座位 {player.Seat}）" : $"对手（座位 {player.Seat}）";
        _stats.Text = $"HP {player.Hp} | 费用 {player.Energy} | 牌库 {player.DeckCount} | 手牌 {player.HandCount}";

        _targetButton.Visible = !isYou;
        _targetButton.Disabled = !awaitingTarget || finished;
        _targetButton.Text = awaitingTarget ? "攻击此英雄" : "选择攻击者后可攻击";
    }
}
