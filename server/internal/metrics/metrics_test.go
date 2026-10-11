package metrics

import (
	"strings"
	"testing"
)

func TestSnapshotCountsAndTextOutput(t *testing.T) {
	registry := New()
	registry.ConnectionOpened()
	registry.ConnectionOpened()
	registry.ConnectionClosed()
	registry.MatchStarted()
	registry.CommandAccepted()
	registry.CommandRejected()
	registry.EventsEmitted(3)
	registry.EventsEmitted(0)
	registry.BroadcastSent()

	want := map[string]int64{
		"riftcards_connections_total":       2,
		"riftcards_connections_active":      1,
		"riftcards_matches_total":           1,
		"riftcards_commands_accepted_total": 1,
		"riftcards_commands_rejected_total": 1,
		"riftcards_events_emitted_total":    3,
		"riftcards_broadcasts_total":        1,
	}

	values := make(map[string]int64)
	for _, sample := range registry.Snapshot() {
		values[sample.Name] = sample.Value
	}
	if len(values) != len(want) {
		t.Fatalf("指标数量 = %d, want %d", len(values), len(want))
	}
	for name, expected := range want {
		if values[name] != expected {
			t.Fatalf("%s = %d, want %d", name, values[name], expected)
		}
	}

	var builder strings.Builder
	if err := registry.WriteText(&builder); err != nil {
		t.Fatalf("WriteText 失败：%v", err)
	}
	text := builder.String()
	if !strings.Contains(text, "riftcards_connections_active 1\n") {
		t.Fatalf("文本输出缺少活动连接行：\n%s", text)
	}
	if lines := strings.Count(strings.TrimSpace(text), "\n") + 1; lines != len(want) {
		t.Fatalf("文本输出 %d 行, want %d：\n%s", lines, len(want), text)
	}
}
