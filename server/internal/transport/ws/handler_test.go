package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/mjq/riftcards/server/internal/game"
	"github.com/mjq/riftcards/server/internal/match"
	"github.com/mjq/riftcards/server/internal/protocol"
)

// testClient 是一个最小的 WebSocket 测试客户端。
type testClient struct {
	t    *testing.T
	ctx  context.Context
	conn *websocket.Conn
}

func dialTestServer(t *testing.T, server *httptest.Server) *testClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	connection, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("WebSocket 拨号失败：%v", err)
	}
	t.Cleanup(func() { _ = connection.Close(websocket.StatusNormalClosure, "done") })
	return &testClient{t: t, ctx: ctx, conn: connection}
}

func (c *testClient) send(envelope protocol.ClientEnvelope) {
	c.t.Helper()
	payload, err := json.Marshal(envelope)
	if err != nil {
		c.t.Fatalf("编码失败：%v", err)
	}
	if err := c.conn.Write(c.ctx, websocket.MessageText, payload); err != nil {
		c.t.Fatalf("发送失败：%v", err)
	}
}

func (c *testClient) sendData(messageType, requestID string, data any) {
	c.t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		c.t.Fatalf("编码载荷失败：%v", err)
	}
	c.send(protocol.ClientEnvelope{Type: messageType, RequestID: requestID, Data: payload})
}

// receivedEnvelope 是测试侧的信封视图：只关心类型、requestId 与原始载荷。
type receivedEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId"`
	Data      json.RawMessage `json:"data"`
}

func (c *testClient) receive() receivedEnvelope {
	c.t.Helper()
	_, payload, err := c.conn.Read(c.ctx)
	if err != nil {
		c.t.Fatalf("接收失败：%v", err)
	}
	var envelope receivedEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		c.t.Fatalf("解析失败：%v", err)
	}
	return envelope
}

// receiveType 跳过其他消息，直到收到指定类型。
func (c *testClient) receiveType(messageType string) receivedEnvelope {
	c.t.Helper()
	for attempt := 0; attempt < 10; attempt++ {
		envelope := c.receive()
		if envelope.Type == messageType {
			return envelope
		}
	}
	c.t.Fatalf("未收到类型为 %s 的消息", messageType)
	return receivedEnvelope{}
}

func (c *testClient) hello(requestID, displayName string) {
	c.t.Helper()
	c.sendData(protocol.TypeHello, requestID, protocol.HelloData{
		ProtocolVersion: protocol.ProtocolVersion,
		ClientVersion:   protocol.ServerVersion,
		DisplayName:     displayName,
	})
	welcome := c.receiveType(protocol.TypeWelcome)
	if welcome.RequestID != requestID {
		c.t.Fatalf("welcome requestId = %q, want %q", welcome.RequestID, requestID)
	}
}

// TestCommandResponseEchoesEnvelopeRequestID 走完整链路：
// hello -> queue.join -> match.started -> match.command，断言响应回填信封 requestId。
func TestCommandResponseEchoesEnvelopeRequestID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewHandler(match.NewManager(), logger, []string{"*"}))
	defer server.Close()

	first := dialTestServer(t, server)
	first.hello("req-hello-1", "One")

	first.sendData(protocol.TypeQueueJoin, "req-queue-1", struct{}{})
	status := first.receiveType(protocol.TypeQueueStatus)
	if status.RequestID != "req-queue-1" {
		t.Fatalf("queue.status requestId = %q, want req-queue-1", status.RequestID)
	}

	second := dialTestServer(t, server)
	second.hello("req-hello-2", "Two")
	second.sendData(protocol.TypeQueueJoin, "req-queue-2", struct{}{})

	started := first.receiveType(protocol.TypeMatchStarted)
	second.receiveType(protocol.TypeMatchStarted)
	var startedPayload protocol.MatchStartedData
	if err := json.Unmarshal(started.Data, &startedPayload); err != nil {
		t.Fatalf("解析 match.started 失败：%v", err)
	}
	if startedPayload.MatchID == "" {
		t.Fatal("match.started 缺少 matchId")
	}
	if started.RequestID != "" {
		t.Fatalf("match.started 是广播，不应回填 requestId，实际 %q", started.RequestID)
	}

	// 过期版本：确定性拒绝，响应必须带上原始 requestId 与 commandId。
	first.sendData(protocol.TypeMatchCommand, "req-cmd-stale", protocol.MatchCommandRequest{
		MatchID:          startedPayload.MatchID,
		CommandID:        "cmd-stale",
		ExpectedRevision: 99,
		Command:          game.PlayerCommand{Type: game.CommandEndTurn},
	})
	stale := first.receiveType(protocol.TypeMatchCommandDone)
	if stale.RequestID != "req-cmd-stale" {
		t.Fatalf("stale 响应 requestId = %q, want req-cmd-stale", stale.RequestID)
	}
	var staleResult protocol.CommandResultData
	if err := json.Unmarshal(stale.Data, &staleResult); err != nil {
		t.Fatalf("解析命令结果失败：%v", err)
	}
	if staleResult.CommandID != "cmd-stale" {
		t.Fatalf("commandId = %q, want cmd-stale", staleResult.CommandID)
	}
	if staleResult.Accepted || staleResult.Error == nil || staleResult.Error.Code != protocol.CodeStaleRevision {
		t.Fatalf("命令结果 = %+v, want stale_revision", staleResult)
	}

	// 版本正确：无论是否当前回合，响应同样回填 requestId。
	first.sendData(protocol.TypeMatchCommand, "req-cmd-ok", protocol.MatchCommandRequest{
		MatchID:          startedPayload.MatchID,
		CommandID:        "cmd-ok",
		ExpectedRevision: startedPayload.State.Revision,
		Command:          game.PlayerCommand{Type: game.CommandEndTurn},
	})
	ok := first.receiveType(protocol.TypeMatchCommandDone)
	if ok.RequestID != "req-cmd-ok" {
		t.Fatalf("响应 requestId = %q, want req-cmd-ok", ok.RequestID)
	}
	var okResult protocol.CommandResultData
	if err := json.Unmarshal(ok.Data, &okResult); err != nil {
		t.Fatalf("解析命令结果失败：%v", err)
	}
	if okResult.CommandID != "cmd-ok" {
		t.Fatalf("commandId = %q, want cmd-ok", okResult.CommandID)
	}
}

// TestProtocolErrorsEchoRequestID 覆盖协议级错误：未知类型与未握手。
func TestProtocolErrorsEchoRequestID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(NewHandler(match.NewManager(), logger, []string{"*"}))
	defer server.Close()

	client := dialTestServer(t, server)

	// 未发送 hello 就提交指令：not_authenticated，且回填 requestId。
	client.sendData(protocol.TypeMatchCommand, "req-before-hello", protocol.MatchCommandRequest{
		MatchID:   "m_1",
		CommandID: "cmd-1",
		Command:   game.PlayerCommand{Type: game.CommandEndTurn},
	})
	unauthorized := client.receiveType(protocol.TypeError)
	if unauthorized.RequestID != "req-before-hello" {
		t.Fatalf("错误 requestId = %q, want req-before-hello", unauthorized.RequestID)
	}
	var unauthorizedPayload protocol.ErrorData
	if err := json.Unmarshal(unauthorized.Data, &unauthorizedPayload); err != nil {
		t.Fatalf("解析错误载荷失败：%v", err)
	}
	if unauthorizedPayload.Code != protocol.CodeNotAuthenticated {
		t.Fatalf("错误码 = %q, want not_authenticated", unauthorizedPayload.Code)
	}

	// 未知消息类型：invalid_message，同样回填 requestId。
	client.sendData("unknown.type", "req-unknown", struct{}{})
	unknown := client.receiveType(protocol.TypeError)
	if unknown.RequestID != "req-unknown" {
		t.Fatalf("错误 requestId = %q, want req-unknown", unknown.RequestID)
	}
	var unknownPayload protocol.ErrorData
	if err := json.Unmarshal(unknown.Data, &unknownPayload); err != nil {
		t.Fatalf("解析错误载荷失败：%v", err)
	}
	if unknownPayload.Code != protocol.CodeInvalidMessage {
		t.Fatalf("错误码 = %q, want invalid_message", unknownPayload.Code)
	}
}
