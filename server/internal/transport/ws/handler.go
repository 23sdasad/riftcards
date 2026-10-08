package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	"github.com/mjq/lscs/server/internal/match"
	"github.com/mjq/lscs/server/internal/protocol"
)

const maxMessageBytes = 64 * 1024

type Handler struct {
	manager        *match.Manager
	logger         *slog.Logger
	allowedOrigins []string
}

func NewHandler(manager *match.Manager, logger *slog.Logger, allowedOrigins []string) *Handler {
	return &Handler{
		manager:        manager,
		logger:         logger,
		allowedOrigins: allowedOrigins,
	}
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		OriginPatterns: h.allowedOrigins,
	})
	if err != nil {
		h.logger.Warn("websocket accept failed", "error", err)
		return
	}
	connection.SetReadLimit(maxMessageBytes)

	ctx := request.Context()
	peer := newPeer(ctx, connection, h.logger)
	defer peer.Close()
	defer h.manager.Disconnect(peer.ID())

	h.readLoop(ctx, peer)
}

func (h *Handler) readLoop(ctx context.Context, peer *peer) {
	for {
		messageType, payload, err := peer.connection.Read(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				h.logger.Debug("websocket read stopped", "peerId", peer.ID(), "error", err)
			}
			return
		}
		if messageType != websocket.MessageText {
			_ = peer.Send(protocol.NewError("", "invalid_message", "only text frames are supported"))
			continue
		}

		envelope, err := protocol.DecodeClientEnvelope(payload)
		if err != nil {
			_ = peer.Send(protocol.NewError("", "invalid_message", err.Error()))
			continue
		}
		h.handleMessage(peer, envelope)
	}
}

func (h *Handler) handleMessage(peer *peer, envelope protocol.ClientEnvelope) {
	switch envelope.Type {
	case protocol.TypeHello:
		if peer.helloDone {
			_ = peer.Send(protocol.NewError(envelope.RequestID, "invalid_message", "hello was already sent"))
			return
		}
		data, err := protocol.DecodeData[protocol.HelloData](envelope.Data)
		if err != nil {
			_ = peer.Send(protocol.NewError(envelope.RequestID, "invalid_message", err.Error()))
			return
		}
		if err := protocol.ValidateHello(data); err != nil {
			_ = peer.Send(protocol.NewError(envelope.RequestID, "unsupported_protocol", err.Error()))
			return
		}
		peer.helloDone = true
		h.manager.Connect(peer, data)

	case protocol.TypeQueueJoin:
		if !h.requireHello(peer, envelope.RequestID) {
			return
		}
		h.manager.JoinQueue(peer.ID())

	case protocol.TypeQueueLeave:
		if !h.requireHello(peer, envelope.RequestID) {
			return
		}
		h.manager.LeaveQueue(peer.ID())

	case protocol.TypeMatchCommand:
		if !h.requireHello(peer, envelope.RequestID) {
			return
		}
		data, err := protocol.DecodeData[protocol.MatchCommandRequest](envelope.Data)
		if err != nil {
			_ = peer.Send(protocol.NewError(envelope.RequestID, "invalid_message", err.Error()))
			return
		}
		if err := protocol.ValidateMatchCommand(data); err != nil {
			_ = peer.Send(protocol.NewError(envelope.RequestID, "invalid_message", err.Error()))
			return
		}
		h.manager.Submit(peer.ID(), data)

	case protocol.TypePing:
		_ = peer.Send(protocol.NewPong(envelope.RequestID))

	default:
		_ = peer.Send(protocol.NewError(envelope.RequestID, "invalid_message", "unsupported message type"))
	}
}

func (h *Handler) requireHello(peer *peer, requestID string) bool {
	if peer.helloDone {
		return true
	}
	_ = peer.Send(protocol.NewError(requestID, "not_authenticated", "send hello first"))
	return false
}

type peer struct {
	connection *websocket.Conn
	logger     *slog.Logger

	id        string
	helloDone bool
	send      chan protocol.ServerEnvelope
	done      chan struct{}
	closeOnce sync.Once
}

func newPeer(ctx context.Context, connection *websocket.Conn, logger *slog.Logger) *peer {
	value := &peer{
		connection: connection,
		logger:     logger,
		id:         randomPeerID(),
		send:       make(chan protocol.ServerEnvelope, 64),
		done:       make(chan struct{}),
	}
	go value.writeLoop(ctx)
	return value
}

func (p *peer) ID() string {
	return p.id
}

func (p *peer) Send(message protocol.ServerEnvelope) error {
	select {
	case <-p.done:
		return errors.New("peer is closed")
	case p.send <- message:
		return nil
	default:
		p.Close()
		return errors.New("peer send queue is full")
	}
}

func (p *peer) Close() {
	p.closeOnce.Do(func() {
		close(p.done)
		if err := p.connection.Close(websocket.StatusNormalClosure, "connection closed"); err != nil {
			p.logger.Debug("websocket close failed", "peerId", p.id, "error", err)
		}
	})
}

func (p *peer) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			p.Close()
			return
		case <-p.done:
			return
		case message := <-p.send:
			payload, err := json.Marshal(message)
			if err != nil {
				p.logger.Error("failed to encode websocket message", "peerId", p.id, "error", err)
				p.Close()
				return
			}
			if err := p.connection.Write(ctx, websocket.MessageText, payload); err != nil {
				p.logger.Debug("websocket write stopped", "peerId", p.id, "error", err)
				p.Close()
				return
			}
		}
	}
}
