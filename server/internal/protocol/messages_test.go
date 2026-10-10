package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mjq/riftcards/server/internal/game"
)

func TestDecodeClientEnvelopeDefaultsEmptyData(t *testing.T) {
	envelope, err := DecodeClientEnvelope([]byte(`{"type":"queue.join","requestId":"req-1"}`))
	if err != nil {
		t.Fatalf("解码失败：%v", err)
	}
	if envelope.RequestID != "req-1" {
		t.Fatalf("requestId = %q, want req-1", envelope.RequestID)
	}
	if string(envelope.Data) != `{}` {
		t.Fatalf("缺失 data 时应补空对象，实际 %s", envelope.Data)
	}
}

func TestDecodeClientEnvelopeRequiresType(t *testing.T) {
	if _, err := DecodeClientEnvelope([]byte(`{"requestId":"req-1"}`)); err == nil {
		t.Fatal("缺少 type 应报错")
	}
}

func TestDecodeClientEnvelopeRejectsUnknownFields(t *testing.T) {
	if _, err := DecodeClientEnvelope([]byte(`{"type":"ping","unexpected":1}`)); err == nil {
		t.Fatal("信封中的未知字段应被拒绝")
	}
}

func TestDecodeDataRejectsUnknownFields(t *testing.T) {
	payload := json.RawMessage(`{"matchId":"m_1","commandId":"c1","expectedRevision":0,"command":{"type":"end_turn"},"extra":true}`)
	if _, err := DecodeData[MatchCommandRequest](payload); err == nil {
		t.Fatal("命令载荷中的未知字段应被拒绝")
	}
}

func TestValidateMatchCommand(t *testing.T) {
	endTurn := game.PlayerCommand{Type: game.CommandEndTurn}
	valid := MatchCommandRequest{
		MatchID:   "m_1",
		CommandID: "c1",
		Command:   endTurn,
	}
	if err := ValidateMatchCommand(valid); err != nil {
		t.Fatalf("合法指令被拒绝：%v", err)
	}

	cases := map[string]MatchCommandRequest{
		"缺少 matchId":   {CommandID: "c1", Command: endTurn},
		"缺少 commandId": {MatchID: "m_1", Command: endTurn},
		"缺少指令类型":       {MatchID: "m_1", CommandID: "c1"},
	}
	for name, request := range cases {
		if err := ValidateMatchCommand(request); err == nil {
			t.Fatalf("%s 应被拒绝", name)
		}
	}
}

func TestValidateHello(t *testing.T) {
	if err := ValidateHello(HelloData{ProtocolVersion: ProtocolVersion, DisplayName: "Tester"}); err != nil {
		t.Fatalf("合法 hello 被拒绝：%v", err)
	}
	if err := ValidateHello(HelloData{ProtocolVersion: "9.9", DisplayName: "Tester"}); err == nil {
		t.Fatal("协议版本不匹配应被拒绝")
	}
	if err := ValidateHello(HelloData{ProtocolVersion: ProtocolVersion}); err == nil {
		t.Fatal("缺少 displayName 应被拒绝")
	}
}

func TestNewErrorCarriesRequestID(t *testing.T) {
	envelope := NewError("req-cmd", CodeNotInMatch, "session is not in a match")
	if envelope.Type != TypeError || envelope.RequestID != "req-cmd" {
		t.Fatalf("信封 = %+v, want type=error requestId=req-cmd", envelope)
	}
	payload, ok := envelope.Data.(ErrorData)
	if !ok || payload.Code != CodeNotInMatch {
		t.Fatalf("载荷 = %#v, want code=not_in_match", envelope.Data)
	}
}

// TestServerEnvelopeOmitsEmptyRequestID 固定契约：广播不带 requestId 字段，
// 响应必须带上，客户端据此关联请求。
func TestServerEnvelopeOmitsEmptyRequestID(t *testing.T) {
	broadcast, err := json.Marshal(NewServerMessage(TypeMatchEvents, "", MatchEventsData{MatchID: "m_1"}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(broadcast), "requestId") {
		t.Fatalf("广播不应包含 requestId：%s", broadcast)
	}

	response, err := json.Marshal(NewServerMessage(TypeMatchCommandDone, "req-cmd", CommandResultData{CommandID: "c1"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(response), `"requestId":"req-cmd"`) {
		t.Fatalf("响应应包含 requestId：%s", response)
	}
}
