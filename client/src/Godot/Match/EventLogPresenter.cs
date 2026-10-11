using System.Text.Json;
using Godot;
using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;

namespace Riftcards.Client.Godot.Match;

/// <summary>
/// 把事件渲染成可读中文句子的表现适配器（约定见 ai-docs/architecture/client-ui.md）。
///
/// 边界：只显示与记录，不改变任何规则状态；未知事件类型标注后忽略。
/// 卡牌显示名从投影里学习（手牌与场地带 `name`），事件载荷里只有 `cardId`/`instanceId`。
/// </summary>
public sealed class EventLogPresenter : IEventPresenter
{
    private const int MaxLines = 200;

    private readonly RichTextLabel _label;
    private readonly GameSession _session;
    private readonly List<string> _lines = [];
    private readonly Dictionary<string, string> _cardNames = new(StringComparer.Ordinal);

    public EventLogPresenter(RichTextLabel label, GameSession session)
    {
        _label = label;
        _session = session;
    }

    public bool Present(GameEvent gameEvent)
    {
        LearnNames();
        _lines.Add(Describe(gameEvent));
        if (_lines.Count > MaxLines)
        {
            _lines.RemoveRange(0, _lines.Count - MaxLines);
        }
        Render();

        // 无界面冒烟时用 stdout 观察事件顺序（保留类型，便于与 seq 对账）。
        GD.Print($"[riftcards] event #{gameEvent.Seq} {gameEvent.Type}");
        return true;
    }

    public void Reset(MatchView? view)
    {
        _lines.Clear();
        _lines.Add(view is null
            ? "表现层已重置"
            : $"已跳到最新快照：第 {view.Turn} 回合，revision {view.Revision}");
        Render();
    }

    private string Describe(GameEvent gameEvent)
    {
        var data = gameEvent.Data;
        var actor = Who(gameEvent.ActorSeat);
        var prefix = $"#{gameEvent.Seq} ";
        return gameEvent.Type switch
        {
            "turn_started" => $"{prefix}第 {Number(data, "turnNumber")} 回合开始：{Who(Number(data, "seat") ?? gameEvent.ActorSeat)}（费用 {Number(data, "energy")}）",
            "turn_ended" => $"{prefix}回合结束：{Who(Number(data, "seat") ?? gameEvent.ActorSeat)}",
            "card_drawn" => $"{prefix}{actor}抽到 {NameOf(data, "cardId")}",
            "card_burned" => $"{prefix}{actor}手牌已满，烧掉 {NameOf(data, "cardId")}",
            "card_played" => $"{prefix}{actor}打出 {NameOf(data, "cardId")}",
            "unit_summoned" => $"{prefix}{actor}召唤 {NameOf(data, "cardId")}（{Number(data, "attack")}/{Number(data, "health")}{GuardText(data)}）",
            "attack_resolved" => DescribeAttack(data, prefix),
            "damage_dealt" => $"{prefix}{TargetName(data)} 受到 {Number(data, "amount")} 点伤害（HP {Number(data, "hpAfter")}）",
            "healed" => $"{prefix}{TargetName(data)} 恢复 {Number(data, "amount")} 点生命（HP {Number(data, "hpAfter")}）",
            "unit_died" => $"{prefix}{NameOf(data, "cardId")} 死亡",
            "fatigue_damage" => $"{prefix}疲劳：{Who(Number(data, "seat") ?? gameEvent.ActorSeat)}受到 {Number(data, "amount")} 点伤害（HP {Number(data, "hpAfter")}）",
            "match_ended" => $"{prefix}对局结束：{WinnerText(data)}（{ReasonText(data)}）",
            _ => $"{prefix}未知事件 {gameEvent.Type}（已忽略）",
        };
    }

    private string DescribeAttack(JsonElement data, string prefix)
    {
        var attacker = NameOf(data, "attackerId");
        var target = TargetName(data);
        var damage = Number(data, "damage");
        if (damage is not null)
        {
            return $"{prefix}{attacker} 攻击 {target}，造成 {damage} 点伤害";
        }
        return $"{prefix}{attacker} 攻击 {target}：造成 {Number(data, "attackerDamage")}，受到 {Number(data, "defenderDamage")}";
    }

    private string WinnerText(JsonElement data)
    {
        var winnerSeat = Number(data, "winnerSeat");
        var youSeat = _session.View?.YouSeat;
        if (winnerSeat is null || youSeat is null)
        {
            return "对局结束";
        }
        return winnerSeat == youSeat ? "你获胜" : "你败北";
    }

    private static string ReasonText(JsonElement data) => JsonText(data, "reason") switch
    {
        "hero_defeated" => "英雄阵亡",
        "surrender" => "认输",
        null => "原因未知",
        var other => other,
    };

    private string Who(int seat)
    {
        var youSeat = _session.View?.YouSeat;
        if (youSeat is null)
        {
            return $"座位 {seat}";
        }
        return seat == youSeat ? "你" : "对手";
    }

    private string TargetName(JsonElement data)
    {
        var targetId = JsonText(data, "targetId");
        if (string.IsNullOrEmpty(targetId))
        {
            return "?";
        }
        if (targetId.StartsWith("hero-", StringComparison.Ordinal)
            && int.TryParse(targetId["hero-".Length..], out var seat))
        {
            return $"{Who(seat)}英雄";
        }
        return _cardNames.TryGetValue(targetId, out var name) ? name : targetId;
    }

    private string NameOf(JsonElement data, string field)
    {
        var id = JsonText(data, field);
        if (string.IsNullOrEmpty(id))
        {
            return "?";
        }
        return _cardNames.TryGetValue(id, out var name) ? name : id;
    }

    private static string GuardText(JsonElement data) =>
        data.ValueKind == JsonValueKind.Object && data.TryGetProperty("guard", out var guard)
        && guard.ValueKind == JsonValueKind.True
            ? "，守卫"
            : string.Empty;

    /// <summary>从投影里学习实例 ID 与卡牌 ID 对应的显示名。</summary>
    private void LearnNames()
    {
        if (_session.View is not { } view)
        {
            return;
        }
        foreach (var player in view.Players)
        {
            foreach (var card in player.Hand)
            {
                Remember(card.InstanceId, card.CardId, card.Name);
            }
            foreach (var unit in player.Board)
            {
                Remember(unit.InstanceId, unit.CardId, unit.Name);
            }
        }
    }

    private void Remember(string instanceId, string cardId, string name)
    {
        if (string.IsNullOrEmpty(name))
        {
            return;
        }
        _cardNames[instanceId] = name;
        _cardNames[cardId] = name;
    }

    private static string? JsonText(JsonElement data, string name) =>
        data.ValueKind == JsonValueKind.Object
        && data.TryGetProperty(name, out var value)
        && value.ValueKind == JsonValueKind.String
            ? value.GetString()
            : null;

    private static int? Number(JsonElement data, string name) =>
        data.ValueKind == JsonValueKind.Object
        && data.TryGetProperty(name, out var value)
        && value.ValueKind == JsonValueKind.Number
        && value.TryGetInt32(out var number)
            ? number
            : null;

    private void Render() => _label.Text = string.Join("\n", _lines);
}
