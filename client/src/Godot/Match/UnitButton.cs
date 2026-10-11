using Godot;
using Riftcards.Client.Core.Protocol;

namespace Riftcards.Client.Godot.Match;

/// <summary>
/// 场地单位按钮：名称 + 攻击/生命 + 守卫徽章 + 剩余攻击次数。
///
/// 只显示服务端投影里的数值：剩余攻击为 0 时弱化是“显示数字”，不是推断原因；
/// 守卫用蓝色边框，选中（作为攻击者）用黄色边框。
/// </summary>
public sealed partial class UnitButton : Button
{
    /// <summary>对应的单位实例 ID。</summary>
    public string InstanceId { get; private set; } = string.Empty;

    public void Configure(UnitView unit, bool opponent, bool selected, bool interactive)
    {
        InstanceId = unit.InstanceId;

        var guard = unit.Guard ? " 守卫" : string.Empty;
        var attacks = opponent ? string.Empty : $" · 攻击 {unit.AttacksRemaining}";
        Text = $"{unit.Name}\n{unit.Attack}/{unit.Health}{guard}{attacks}";
        TooltipText = opponent ? "对手单位：作为攻击目标" : "己方单位：点击选择攻击者";

        CustomMinimumSize = new Vector2(UiTheme.UnitWidth, UiTheme.UnitHeight);
        Disabled = !interactive;

        var border = selected ? UiTheme.Selection : unit.Guard ? UiTheme.Guard : UiTheme.PanelBorder;
        UiTheme.StyleButton(this, border, selected);

        // 剩余攻击为 0：整体弱化，提醒本回合无法再攻击。
        Modulate = !opponent && unit.AttacksRemaining <= 0 ? new Color(1f, 1f, 1f, 0.55f) : Colors.White;
    }
}
