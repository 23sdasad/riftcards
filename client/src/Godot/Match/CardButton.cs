using Godot;
using Riftcards.Client.Core.Protocol;

namespace Riftcards.Client.Godot.Match;

/// <summary>
/// 手牌按钮：名称 + 类型与费用（两行居中）。
///
/// 不做费用可用性着色：那需要客户端计算费用合法性，违反 008 的边界；
/// 费用不足由服务端拒绝并显示错误码。只有在不是自己回合或对局结束时才禁用。
/// </summary>
public sealed partial class CardButton : Button
{
    /// <summary>对应的手牌实例 ID。</summary>
    public string InstanceId { get; private set; } = string.Empty;

    public void Configure(CardView card, bool selected, bool interactive)
    {
        InstanceId = card.InstanceId;

        var kind = card.Kind == ProtocolNames.CardKindSpell ? "法术" : "单位";
        Text = $"{card.Name}\n{kind} · {card.Cost} 费";
        TooltipText = card.Kind == ProtocolNames.CardKindSpell
            ? "法术：点击后选择目标"
            : "单位：点击后召唤";

        CustomMinimumSize = new Vector2(UiTheme.CardWidth, UiTheme.CardHeight);
        Disabled = !interactive;
        UiTheme.StyleButton(this, selected ? UiTheme.Selection : UiTheme.PanelBorder, selected);
    }
}
