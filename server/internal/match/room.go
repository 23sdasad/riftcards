package match

import (
	"fmt"
	"sync"

	"github.com/mjq/riftcards/server/internal/game"
	"github.com/mjq/riftcards/server/internal/protocol"
)

type Room struct {
	id string

	mu        sync.Mutex
	state     *game.State
	peers     [2]Peer
	eventLog  []game.Event
	started   bool
	finished  bool
}

func NewRoom(id string, seed int64, peers [2]Peer) (*Room, error) {
	state, _ := game.NewMatch(id, seed)
	return &Room{
		id:    id,
		state: state,
		peers: peers,
	}, nil
}

func (r *Room) ID() string {
	return r.id
}

func (r *Room) Start() {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.started = true
	views := [2]game.MatchView{
		r.state.ViewForSeat(0),
		r.state.ViewForSeat(1),
	}
	lastEventSeq := r.state.LastEventSeq
	peers := r.peers
	r.mu.Unlock()

	for seat := 0; seat < 2; seat++ {
		_ = peers[seat].Send(protocol.NewServerMessage(protocol.TypeMatchStarted, "", protocol.MatchStartedData{
			MatchID: r.id,
			Seat:    seat,
			State:   views[seat],
		}))
	}
}

func (r *Room) Submit(seat int, request protocol.MatchCommandRequest) {
	r.mu.Lock()
	if request.MatchID != r.id {
		r.mu.Unlock()
		_ = r.peers[seat].Send(protocol.NewError("", "not_in_match", "matchId does not belong to this session"))
		return
	}
	if request.ExpectedRevision != r.state.Revision {
		revision := r.state.Revision
		r.mu.Unlock()
		_ = r.peers[seat].Send(protocol.NewServerMessage(protocol.TypeMatchCommandDone, "", protocol.CommandResultData{
			CommandID: request.CommandID,
			Accepted:  false,
			Revision:  revision,
			Error: &game.CommandError{
				Code:    "stale_revision",
				Message: fmt.Sprintf("expected revision %d, current revision is %d", request.ExpectedRevision, revision),
			},
		}))
		return
	}

	baseRevision := r.state.Revision
	result := r.state.ApplyCommand(seat, request.Command)
	if result.Accepted {
		r.eventLog = append(r.eventLog, result.Events...)
		if r.state.Status == game.StatusFinished {
			r.finished = true
		}
	}
	events := append([]game.Event(nil), result.Events...)
	views := [2]game.MatchView{
		r.state.ViewForSeat(0),
		r.state.ViewForSeat(1),
	}
	peers := r.peers
	r.mu.Unlock()

	_ = peers[seat].Send(protocol.NewServerMessage(protocol.TypeMatchCommandDone, "", protocol.CommandResultData{
		CommandID: request.CommandID,
		Accepted:  result.Accepted,
		Revision:  result.Revision,
		Error:     result.Error,
	}))

	if !result.Accepted || len(events) == 0 {
		return
	}

	for eventSeat := 0; eventSeat < 2; eventSeat++ {
		visibleEvents := game.EventsForSeat(events, eventSeat)
		_ = peers[eventSeat].Send(protocol.NewServerMessage(protocol.TypeMatchEvents, "", protocol.MatchEventsData{
			MatchID:      r.id,
			BaseRevision: baseRevision,
			Revision:     result.Revision,
			LastEventSeq: lastEventSeq,
			Events:       visibleEvents,
			State:        views[eventSeat],
		}))
	}
}

func (r *Room) Forfeit(seat int, reason string) {
	if seat < 0 || seat > 1 {
		return
	}

	r.mu.Lock()
	if r.finished {
		r.mu.Unlock()
		return
	}
	baseRevision := r.state.Revision
	result := r.state.ApplyCommand(seat, game.PlayerCommand{Type: game.CommandSurrender})
	if result.Accepted {
		r.eventLog = append(r.eventLog, result.Events...)
		r.finished = true
	}
	events := append([]game.Event(nil), result.Events...)
	views := [2]game.MatchView{
		r.state.ViewForSeat(0),
		r.state.ViewForSeat(1),
	}
	lastEventSeq := r.state.LastEventSeq
	peers := r.peers
	r.mu.Unlock()

	if !result.Accepted || len(events) == 0 {
		return
	}

	for eventSeat := 0; eventSeat < 2; eventSeat++ {
		eventData := make([]map[string]any, 0, len(events))
		for _, event := range game.EventsForSeat(events, eventSeat) {
			eventData = append(eventData, map[string]any{
				"seq":        event.Seq,
				"revision":   event.Revision,
				"type":       event.Type,
				"actorSeat":  event.ActorSeat,
				"visibility": event.Visibility,
				"data":       event.Data,
			})
		}
		_ = peers[eventSeat].Send(protocol.NewServerMessage(protocol.TypeMatchEvents, "", map[string]any{
			"matchId":      r.id,
			"baseRevision": baseRevision,
			"revision":     result.Revision,
			"lastEventSeq": lastEventSeq,
			"events":       eventData,
			"state":        views[eventSeat],
			"reason":       reason,
		}))
	}
}

func (r *Room) ReplayEvents() []game.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]game.Event(nil), r.eventLog...)
}
