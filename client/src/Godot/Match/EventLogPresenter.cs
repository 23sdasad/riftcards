using Godot;
using Riftcards.Client.Core.Protocol;
using Riftcards.Client.Core.Session;

namespace Riftcards.Client.Godot.Match;

/// <summary>
/// 把事件渲染成日志行的表现适配器。
///
/// 边界：只显示与记录，不改变任何规则状态；未知事件类型标注后忽略。
/// </summary>
public sealed class EventLogPresenter : IEventPresenter
{
    private const int MaxLines = 80;

    private readonly RichTextLabel _label;
    private readonly List<string> _lines = [];

    public EventLogPresenter(RichTextLabel label)
    {
        _label = label;
    }

    public bool Present(GameEvent gameEvent)
    {
        var known = ProtocolNames.KnownEventTypes.Contains(gameEvent.Type);
        _lines.Add(Describe(gameEvent, known));
        if (_lines.Count > MaxLines)
        {
            _lines.RemoveRange(0, _lines.Count - MaxLines);
        }
        Render();

        // 无界面冒烟时用 stdout 观察事件顺序。
        GD.Print($"[riftcards] event #{gameEvent.Seq} {gameEvent.Type}{(known ? string.Empty : " (unknown, ignored)")}");
        return true;
    }

    public void Reset(MatchView? view)
    {
        _lines.Clear();
        _lines.Add(view is null
            ? "表现层已重置"
            : $"已跳到最新快照：revision {view.Revision}，lastEventSeq {view.LastEventSeq}");
        Render();
    }

    private static string Describe(GameEvent gameEvent, bool known) =>
        $"#{gameEvent.Seq} r{gameEvent.Revision} {gameEvent.Type}{(known ? string.Empty : "（未知类型，已忽略）")} {gameEvent.Data}";

    private void Render() => _label.Text = string.Join("\n", _lines);
}
