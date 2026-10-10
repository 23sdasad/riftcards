package match

import (
	"sync"

	"github.com/mjq/riftcards/server/internal/protocol"
)

// fakePeer 记录服务端发出的所有信封，供房间与队列测试断言。
type fakePeer struct {
	id string

	mu       sync.Mutex
	messages []protocol.ServerEnvelope
}

func newFakePeer(id string) *fakePeer {
	return &fakePeer{id: id}
}

func (p *fakePeer) ID() string {
	return p.id
}

func (p *fakePeer) Send(message protocol.ServerEnvelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = append(p.messages, message)
	return nil
}

// all 返回已发送消息的副本。
func (p *fakePeer) all() []protocol.ServerEnvelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.ServerEnvelope(nil), p.messages...)
}

// byType 返回指定类型的已发送消息。
func (p *fakePeer) byType(messageType string) []protocol.ServerEnvelope {
	var matched []protocol.ServerEnvelope
	for _, message := range p.all() {
		if message.Type == messageType {
			matched = append(matched, message)
		}
	}
	return matched
}

// reset 清空已记录的消息。
func (p *fakePeer) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = nil
}
