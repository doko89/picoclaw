package missioncontrol

import (
	"encoding/json"
	"sync"
)

// SSEEvent represents a server-sent event broadcast to all connected clients.
type SSEEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Broadcaster manages SSE client subscriptions and broadcasts events.
type Broadcaster struct {
	mu      sync.RWMutex
	clients map[chan SSEEvent]struct{}
}

// NewBroadcaster creates a new Broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[chan SSEEvent]struct{}),
	}
}

// Subscribe registers a new SSE client and returns its event channel.
// The caller must call Unsubscribe when done to prevent leaks.
func (b *Broadcaster) Subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 32)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel and closes it.
func (b *Broadcaster) Unsubscribe(ch chan SSEEvent) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	// Drain and close under lock-free context
	select {
	case <-ch:
	default:
	}
	close(ch)
}

// Broadcast sends an event to all subscribed clients. Slow clients
// (full buffer) are skipped to prevent blocking.
func (b *Broadcaster) Broadcast(event SSEEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default:
			// Client is too slow; drop event to avoid blocking.
		}
	}
}

// ActiveCount returns the number of currently subscribed clients.
func (b *Broadcaster) ActiveCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// SSEBroadcast is the global SSE broadcast function.
// Set by main.go after the broadcaster is created.
var SSEBroadcast func(eventType string, payload map[string]interface{})

// BroadcastEvent is a helper that calls SSEBroadcast if set.
func BroadcastEvent(eventType string, payload map[string]interface{}) {
	if SSEBroadcast != nil {
		SSEBroadcast(eventType, payload)
	}
}