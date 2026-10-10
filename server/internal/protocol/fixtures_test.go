package protocol

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mjq/riftcards/server/internal/game"
	"github.com/mjq/riftcards/server/internal/testfixtures"
)

// sampleEnvelope 是共享样例里的信封视图。
type sampleEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId"`
	Data      json.RawMessage `json:"data"`
}

func decodeSampleEnvelope(t *testing.T, raw json.RawMessage) sampleEnvelope {
	t.Helper()
	var envelope sampleEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("解析样例信封失败：%v", err)
	}
	if envelope.Type == "" {
		t.Fatal("样例缺少 type")
	}
	return envelope
}

// assertRoundTrip 断言 DTO 再编码后仍能严格解析为等价结构（解析与生成一致）。
func assertRoundTrip[T any](t *testing.T, value T) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("编码失败：%v", err)
	}
	decoded, err := DecodeData[T](encoded)
	if err != nil {
		t.Fatalf("再解析失败：%v", err)
	}
	if !reflect.DeepEqual(value, decoded) {
		t.Fatalf("往返不等价：\n原值=%+v\n再解析=%+v", value, decoded)
	}
}

// TestServerFixturesMatchDTOs 用共享样例验证服务端 DTO 能解析并回编码。
func TestServerFixturesMatchDTOs(t *testing.T) {
	file, err := testfixtures.Load("server-messages.json")
	if err != nil {
		t.Fatalf("读取样例失败：%v", err)
	}
	if file.Version != ProtocolVersion {
		t.Fatalf("样例版本 = %q, want %q", file.Version, ProtocolVersion)
	}

	for _, sample := range file.Messages {
		t.Run(sample.Name, func(t *testing.T) {
			envelope := decodeSampleEnvelope(t, sample.Message)

			// 契约：响应必须带 requestId，广播不带。
			isBroadcast := envelope.Type == TypeMatchStarted || envelope.Type == TypeMatchEvents
			if isBroadcast && envelope.RequestID != "" {
				t.Fatalf("%s 是广播，样例不应带 requestId", envelope.Type)
			}
			if !isBroadcast && envelope.RequestID == "" {
				t.Fatalf("%s 是响应，样例必须带 requestId", envelope.Type)
			}

			switch envelope.Type {
			case TypeWelcome:
				payload, err := DecodeData[WelcomeData](envelope.Data)
				if err != nil {
					t.Fatalf("解析 welcome 失败：%v", err)
				}
				if payload.ConnectionID != "c_4a91" || payload.PlayerID != "p_01f2" || payload.DisplayName != "Player One" {
					t.Fatalf("welcome 载荷 = %+v", payload)
				}
				if payload.ProtocolVersion != ProtocolVersion || payload.ServerVersion != ServerVersion {
					t.Fatalf("welcome 版本 = %s/%s", payload.ServerVersion, payload.ProtocolVersion)
				}
				assertRoundTrip(t, payload)

			case TypeQueueStatus:
				payload, err := DecodeData[QueueStatusData](envelope.Data)
				if err != nil {
					t.Fatalf("解析 queue.status 失败：%v", err)
				}
				if payload.State != "waiting" && payload.State != "left" {
					t.Fatalf("queue.status state = %q", payload.State)
				}
				assertRoundTrip(t, payload)

			case TypeMatchStarted:
				payload, err := DecodeData[MatchStartedData](envelope.Data)
				if err != nil {
					t.Fatalf("解析 match.started 失败：%v", err)
				}
				if payload.MatchID != "m_9a2f" || payload.Seat != 0 {
					t.Fatalf("match.started = %+v", payload)
				}
				if len(payload.State.Players) != 2 {
					t.Fatalf("投影玩家数 = %d, want 2", len(payload.State.Players))
				}
				if len(payload.State.Players[0].Hand) != 1 || payload.State.Players[0].Hand[0].CardID != "ember_squire" {
					t.Fatalf("本方手牌 = %+v", payload.State.Players[0].Hand)
				}
				if len(payload.State.Players[1].Hand) != 0 || payload.State.Players[1].HandCount != 4 {
					t.Fatalf("对手手牌泄露：%+v", payload.State.Players[1])
				}
				assertRoundTrip(t, payload)

			case TypeMatchCommandDone:
				payload, err := DecodeData[CommandResultData](envelope.Data)
				if err != nil {
					t.Fatalf("解析 match.command_result 失败：%v", err)
				}
				if payload.CommandID == "" {
					t.Fatal("命令结果缺少 commandId")
				}
				if payload.Accepted && payload.Error != nil {
					t.Fatalf("接受的结果不应带错误：%+v", payload.Error)
				}
				if !payload.Accepted && payload.Error == nil {
					t.Fatal("拒绝的结果必须带错误码")
				}
				assertRoundTrip(t, payload)

			case TypeMatchEvents:
				payload, err := DecodeData[MatchEventsData](envelope.Data)
				if err != nil {
					t.Fatalf("解析 match.events 失败：%v", err)
				}
				if payload.MatchID != "m_9a2f" || payload.BaseRevision != 0 || payload.Revision != 1 {
					t.Fatalf("match.events = %+v", payload)
				}
				if len(payload.Events) != 2 || payload.Events[0].Seq != 1 || payload.Events[1].Type != "unit_summoned" {
					t.Fatalf("事件序列 = %+v", payload.Events)
				}
				if payload.State.Revision != payload.Revision {
					t.Fatalf("投影版本 %d 与广播版本 %d 不一致", payload.State.Revision, payload.Revision)
				}
				assertRoundTrip(t, payload)

			case TypeError:
				payload, err := DecodeData[ErrorData](envelope.Data)
				if err != nil {
					t.Fatalf("解析 error 失败：%v", err)
				}
				if payload.Code == "" || payload.Message == "" {
					t.Fatalf("error 载荷不完整：%+v", payload)
				}
				assertRoundTrip(t, payload)

			case TypePong:
				payload, err := DecodeData[PongData](envelope.Data)
				if err != nil {
					t.Fatalf("解析 pong 失败：%v", err)
				}
				if payload.ServerTime == "" {
					t.Fatal("pong 缺少 serverTime")
				}
				assertRoundTrip(t, payload)

			default:
				t.Fatalf("样例包含未覆盖的服务端消息类型 %q", envelope.Type)
			}
		})
	}
}

// TestClientFixturesMatchDTOs 用共享样例验证客户端消息能被 Go 严格解析并通过校验。
func TestClientFixturesMatchDTOs(t *testing.T) {
	file, err := testfixtures.Load("client-messages.json")
	if err != nil {
		t.Fatalf("读取样例失败：%v", err)
	}

	seen := map[string]bool{}
	for _, sample := range file.Messages {
		t.Run(sample.Name, func(t *testing.T) {
			envelope := decodeSampleEnvelope(t, sample.Message)
			if envelope.RequestID == "" {
				t.Fatal("客户端请求必须带 requestId")
			}
			seen[envelope.Type] = true

			switch envelope.Type {
			case TypeHello:
				payload, err := DecodeData[HelloData](envelope.Data)
				if err != nil {
					t.Fatalf("解析 hello 失败：%v", err)
				}
				if err := ValidateHello(payload); err != nil {
					t.Fatalf("hello 未通过校验：%v", err)
				}
				if payload.DisplayName != "Player One" {
					t.Fatalf("displayName = %q", payload.DisplayName)
				}
				assertRoundTrip(t, payload)

			case TypeQueueJoin, TypeQueueLeave, TypePing:
				if string(envelope.Data) != `{}` {
					t.Fatalf("%s 的载荷应为空对象，实际 %s", envelope.Type, envelope.Data)
				}

			case TypeMatchCommand:
				payload, err := DecodeData[MatchCommandRequest](envelope.Data)
				if err != nil {
					t.Fatalf("解析 match.command 失败：%v", err)
				}
				if err := ValidateMatchCommand(payload); err != nil {
					t.Fatalf("match.command 未通过校验：%v", err)
				}
				if payload.MatchID != "m_9a2f" || payload.CommandID == "" || payload.Command.Type == "" {
					t.Fatalf("match.command = %+v", payload)
				}
				assertRoundTrip(t, payload)

			default:
				t.Fatalf("样例包含未覆盖的客户端消息类型 %q", envelope.Type)
			}
		})
	}

	for _, messageType := range []string{TypeHello, TypeQueueJoin, TypeQueueLeave, TypeMatchCommand, TypePing} {
		if !seen[messageType] {
			t.Fatalf("样例缺少客户端消息 %s", messageType)
		}
	}
}

// TestGeneratedCommandMatchesFixture 验证 Go 构造的指令与样例逐字段一致（生成方向）。
func TestGeneratedCommandMatchesFixture(t *testing.T) {
	file, err := testfixtures.Load("client-messages.json")
	if err != nil {
		t.Fatalf("读取样例失败：%v", err)
	}

	var found bool
	for _, sample := range file.Messages {
		if sample.Name != "match.command.play_card" {
			continue
		}
		found = true

		envelope := decodeSampleEnvelope(t, sample.Message)
		fromFixture, err := DecodeData[MatchCommandRequest](envelope.Data)
		if err != nil {
			t.Fatalf("解析样例失败：%v", err)
		}

		generated := MatchCommandRequest{
			MatchID:          "m_9a2f",
			CommandID:        "cmd-001",
			ExpectedRevision: 0,
			Command: game.PlayerCommand{
				Type:           game.CommandPlayCard,
				CardInstanceID: "p0-c0",
			},
		}
		if !reflect.DeepEqual(fromFixture, generated) {
			t.Fatalf("Go 生成的指令与样例不一致：\n样例=%+v\n生成=%+v", fromFixture, generated)
		}
		assertRoundTrip(t, generated)
	}
	if !found {
		t.Fatal("样例缺少 match.command.play_card")
	}
}
