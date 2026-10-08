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

func (m *Manager) Connect(peer Peer, hello protocol.HelloData) {
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

	_ = peer.Send(protocol.NewServerMessage(protocol.TypeWelcome, "", protocol.WelcomeData{
		ConnectionID:    peer.ID(),
		PlayerID:        playerID,
		DisplayName:     hello.DisplayName,
		ServerVersion:   protocol.ServerVersion,
		ProtocolVersion: protocol.ProtocolVersion,
	}))
}

func (m *Manager) JoinQueue(sessionID string) {
	m.mu.Lock()
	current, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return
	}
	if current.room != nil {
		m.mu.Unlock()
		_ = current.peer.Send(protocol.NewError("", "invalid_message", "session is already in a match"))
		return
	}
	for _, queuedID := range m.waiting {
		if queuedID == sessionID {
			m.mu.Unlock()
			_ = current.peer.Send(protocol.NewServerMessage(protocol.TypeQueueStatus, "", protocol.QueueStatusData{
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
		_ = current.peer.Send(protocol.NewServerMessage(protocol.TypeQueueStatus, "", protocol.QueueStatusData{
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
		_ = first.peer.Send(protocol.NewError("", "internal_error", "failed to create match"))
		_ = second.peer.Send(protocol.NewError("", "internal_error", "failed to create match"))
		return
	}
	first.room = room
	second.room = room
	m.rooms[room.ID()] = room
	m.mu.Unlock()

	room.Start()
}

func (m *Manager) LeaveQueue(sessionID string) {
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
		_ = current.peer.Send(protocol.NewServerMessage(protocol.TypeQueueStatus, "", protocol.QueueStatusData{
			State: "left",
		}))
	}
}

func (m *Manager) Submit(sessionID string, request protocol.MatchCommandRequest) {
	m.mu.Lock()
	current, ok := m.sessions[sessionID]
	if !ok || current.room == nil {
		m.mu.Unlock()
		return
	}
	room := current.room
	seat := current.seat
	m.mu.Unlock()

	room.Submit(seat, request)
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
		room.Forfeit(seat, "disconnect")
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
