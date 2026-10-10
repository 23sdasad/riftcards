package match

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/mjq/riftcards/server/internal/protocol"
)

type Peer interface {
	ID() string
	Send(message protocol.ServerEnvelope) error
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
	waiting  []string
	rooms    map[string]*Room
}

type session struct {
	id    string
	peer  Peer
	hello protocol.HelloData
	seat  int
	room  *Room
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*session),
		rooms:    make(map[string]*Room),
	}
}

// Connect 建立会话并回复 welcome。
// requestID 来自客户端信封，凡是请求/响应式的消息都必须原样回填。
func (m *Manager) Connect(peer Peer, requestID string, hello protocol.HelloData) {
	playerID := randomID("p")
	session := &session{
		id:    peer.ID(),
		peer:  peer,
		hello: hello,
		seat:  -1,
	}

	m.mu.Lock()
	m.sessions[session.id] = session
	m.mu.Unlock()

	_ = peer.Send(protocol.NewServerMessage(protocol.TypeWelcome, requestID, protocol.WelcomeData{
		ConnectionID:    peer.ID(),
		PlayerID:        playerID,
		DisplayName:     hello.DisplayName,
		ServerVersion:   protocol.ServerVersion,
		ProtocolVersion: protocol.ProtocolVersion,
	}))
}

// JoinQueue 把会话放入匹配队列；凑满两人时创建房间并开始对局。
// 队列状态响应回填调用方的 requestID；match.started 是广播，不带 requestID。
func (m *Manager) JoinQueue(sessionID, requestID string) {
	m.mu.Lock()
	current, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return
	}
	if current.room != nil {
		m.mu.Unlock()
		_ = current.peer.Send(protocol.NewError(requestID, protocol.CodeInvalidMessage, "session is already in a match"))
		return
	}
	for _, queuedID := range m.waiting {
		if queuedID == sessionID {
			m.mu.Unlock()
			_ = current.peer.Send(protocol.NewServerMessage(protocol.TypeQueueStatus, requestID, protocol.QueueStatusData{
				State:    "waiting",
				Position: 1,
			}))
			return
		}
	}

	m.waiting = append(m.waiting, sessionID)
	if len(m.waiting) < 2 {
		position := len(m.waiting)
		m.mu.Unlock()
		_ = current.peer.Send(protocol.NewServerMessage(protocol.TypeQueueStatus, requestID, protocol.QueueStatusData{
			State:    "waiting",
			Position: position,
		}))
		return
	}

	first := m.sessions[m.waiting[0]]
	second := m.sessions[m.waiting[1]]
	m.waiting = m.waiting[2:]
	first.seat = 0
	second.seat = 1

	room, err := NewRoom(randomID("m"), randomSeed(), [2]Peer{first.peer, second.peer})
	if err != nil {
		delete(m.sessions, first.id)
		delete(m.sessions, second.id)
		m.mu.Unlock()
		_ = first.peer.Send(protocol.NewError(requestID, protocol.CodeInternalError, "failed to create match"))
		_ = second.peer.Send(protocol.NewError(requestID, protocol.CodeInternalError, "failed to create match"))
		return
	}
	first.room = room
	second.room = room
	m.rooms[room.ID()] = room
	m.mu.Unlock()

	room.Start()
}

// LeaveQueue 把会话移出队列，并回复离开状态。
func (m *Manager) LeaveQueue(sessionID, requestID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for index, queuedID := range m.waiting {
		if queuedID != sessionID {
			continue
		}
		m.waiting = append(m.waiting[:index], m.waiting[index+1:]...)
		break
	}

	if current, ok := m.sessions[sessionID]; ok {
		_ = current.peer.Send(protocol.NewServerMessage(protocol.TypeQueueStatus, requestID, protocol.QueueStatusData{
			State: "left",
		}))
	}
}

// Submit 把一条 match.command 转交给会话所在房间。
// 会话不在对局中时必须显式拒绝：否则客户端拿不到任何响应，只能等超时。
func (m *Manager) Submit(sessionID, requestID string, request protocol.MatchCommandRequest) {
	m.mu.Lock()
	current, ok := m.sessions[sessionID]
	if !ok || current.room == nil {
		m.mu.Unlock()
		if ok {
			_ = current.peer.Send(protocol.NewError(requestID, protocol.CodeNotInMatch, "session is not in a match"))
		}
		return
	}
	room := current.room
	seat := current.seat
	m.mu.Unlock()

	room.Submit(seat, requestID, request)
}

func (m *Manager) Disconnect(sessionID string) {
	m.mu.Lock()
	current, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return
	}
	delete(m.sessions, sessionID)
	for index, queuedID := range m.waiting {
		if queuedID == sessionID {
			m.waiting = append(m.waiting[:index], m.waiting[index+1:]...)
			break
		}
	}
	room := current.room
	seat := current.seat
	m.mu.Unlock()

	if room != nil {
		// MVP 中连接断开按认输处理（规则 10）：由房间产生 match_ended 事件。
		room.Forfeit(seat)
	}
}

func randomID(prefix string) string {
	var value [6]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return prefix + "_" + hex.EncodeToString(value[:])
}

func randomSeed() int64 {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return int64(binary.LittleEndian.Uint64(value[:]))
}
