using Godot;
using Riftcards.Client.Core.Protocol;

namespace Riftcards.Client.Godot.Match;

/// <summary>场地单位按钮：显示投影数值，己方单位用于选择攻击者，对手单位作为攻击目标。</summary>
public sealed partial class UnitButton : Button
{
    /// <summary>对应的单位实例 ID。</summary>
    public string InstanceId { get; private set; } = string.Empty;

    public void Configure(UnitView unit, bool opponent, bool selected, bool interactive)
    {
        InstanceId = unit.InstanceId;
        var guard = unit.Guard ? " 守卫" : string.Empty;
        var attacks = opponent ? string.Empty : $" 攻击 {unit.AttacksRemaining}";
        Text = $"{unit.Name} {unit.Attack}/{unit.Health}{guard}{attacks}";
        TooltipText = opponent ? "对手单位：作为攻击目标" : "己方单位：点击选择攻击者";
        Disabled = !interactive;
        Modulate = selected ? new Color(1.0f, 0.9f, 0.5f) : Colors.White;
    }
}
