package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/mjq/riftcards/server/internal/game"
)

const (
	ProtocolVersion = "0.1"
	ServerVersion   = "0.1.0"

	TypeHello            = "hello"
	TypeQueueJoin        = "queue.join"
	TypeQueueLeave       = "queue.leave"
	TypeMatchCommand     = "match.command"
	TypePing             = "ping"
	TypeWelcome          = "welcome"
	TypeQueueStatus      = "queue.status"
	TypeMatchStarted     = "match.started"
	TypeMatchCommandDone = "match.command_result"
	TypeMatchEvents      = "match.events"
	TypeError            = "error"
	TypePong             = "pong"
)

type ClientEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	Data      json.RawMessage `json:"data"`
}

type ServerEnvelope struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Data      any    `json:"data"`
}

type HelloData struct {
	ProtocolVersion string `json:"protocolVersion"`
	ClientVersion   string `json:"clientVersion"`
	DisplayName     string `json:"displayName"`
}

type WelcomeData struct {
	ConnectionID    string `json:"connectionId"`
	PlayerID        string `json:"playerId"`
	DisplayName     string `json:"displayName"`
	ServerVersion   string `json:"serverVersion"`
	ProtocolVersion string `json:"protocolVersion"`
}

type QueueStatusData struct {
	State    string `json:"state"`
	Position int    `json:"position,omitempty"`
}

type MatchCommandRequest struct {
	MatchID          string              `json:"matchId"`
	CommandID        string              `json:"commandId"`
	ExpectedRevision int64               `json:"expectedRevision"`
	Command          game.PlayerCommand  `json:"command"`
}

type MatchStartedData struct {
	MatchID string         `json:"matchId"`
	Seat    int            `json:"seat"`
	State   game.MatchView `json:"state"`
}

type CommandResultData struct {
	CommandID string              `json:"commandId"`
	Accepted  bool                `json:"accepted"`
	Revision  int64               `json:"revision"`
	Error     *game.CommandError  `json:"error"`
}

type MatchEventsData struct {
	MatchID      string         `json:"matchId"`
	BaseRevision int64          `json:"baseRevision"`
	Revision     int64          `json:"revision"`
	LastEventSeq int64          `json:"lastEventSeq"`
	Events       []game.Event   `json:"events"`
	State        game.MatchView `json:"state"`
}

type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PongData struct {
	ServerTime string `json:"serverTime"`
}

func NewServerMessage(messageType, requestID string, data any) ServerEnvelope {
	return ServerEnvelope{
		Type:      messageType,
		RequestID: requestID,
		Data:      data,
	}
}

func NewError(requestID, code, message string) ServerEnvelope {
	return NewServerMessage(TypeError, requestID, ErrorData{
		Code:    code,
		Message: message,
	})
}

func NewPong(requestID string) ServerEnvelope {
	return NewServerMessage(TypePong, requestID, PongData{
		ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func DecodeClientEnvelope(data []byte) (ClientEnvelope, error) {
	var envelope ClientEnvelope
	if err := decodeStrict(data, &envelope); err != nil {
		return ClientEnvelope{}, err
	}
	if envelope.Type == "" {
		return ClientEnvelope{}, fmt.Errorf("missing type")
	}
	if len(envelope.Data) == 0 {
		envelope.Data = json.RawMessage(`{}`)
	}
	return envelope, nil
}

func DecodeData[T any](data json.RawMessage) (T, error) {
	var value T
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	if err := decodeStrict(data, &value); err != nil {
		return value, err
	}
	return value, nil
}

func ValidateHello(data HelloData) error {
	if data.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported protocol version %q, expected %q", data.ProtocolVersion, ProtocolVersion)
	}
	if data.DisplayName == "" {
		return fmt.Errorf("displayName is required")
	}
	return nil
}

func ValidateMatchCommand(data MatchCommandRequest) error {
	if data.MatchID == "" {
		return fmt.Errorf("matchId is required")
	}
	if data.CommandID == "" {
		return fmt.Errorf("commandId is required")
	}
	if data.Command.Type == "" {
		return fmt.Errorf("command.type is required")
	}
	return nil
}

func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}
