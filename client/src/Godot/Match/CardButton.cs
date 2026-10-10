using Godot;
using Riftcards.Client.Core.Protocol;

namespace Riftcards.Client.Godot.Match;

/// <summary>手牌按钮：显示名称与费用，点击后交给视图模型决定是立即出牌还是等待目标。</summary>
public sealed partial class CardButton : Button
{
    /// <summary>对应的手牌实例 ID。</summary>
    public string InstanceId { get; private set; } = string.Empty;

    public void Configure(CardView card, bool selected, bool interactive)
    {
        InstanceId = card.InstanceId;
        Text = $"{card.Name} ({card.Cost})";
        TooltipText = card.Kind == ProtocolNames.CardKindSpell
            ? "法术：点击后选择目标"
            : "单位：点击后召唤";
        Disabled = !interactive;
        Modulate = selected ? new Color(1.0f, 0.9f, 0.5f) : Colors.White;
    }
}
