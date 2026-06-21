package chat

import (
	"context"
	"log/slog"
	"sync"
)

type presenceChange struct {
	userID string
	online bool
}

// Hub maintains the set of active WebSocket clients keyed by user ID.
// At most one connection is allowed per user; a new connection displaces the old one.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client

	register   chan *Client
	unregister chan *Client
	presenceC  chan presenceChange

	log *slog.Logger
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 32),
		unregister: make(chan *Client, 32),
		presenceC:  make(chan presenceChange, 64),
		log:        log,
	}
}

// IsOnline reports whether the user currently has an active WS connection.
func (h *Hub) IsOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

// PresenceC returns the channel that emits connect/disconnect events.
func (h *Hub) PresenceC() <-chan presenceChange {
	return h.presenceC
}

// Run processes hub events until ctx is cancelled, then closes all connections.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for _, c := range h.clients {
				c.shutdown()
			}
			h.mu.Unlock()
			return
		case c := <-h.register:
			h.mu.Lock()
			if old, ok := h.clients[c.userID]; ok {
				old.shutdown() // displace any existing connection for this user
			}
			h.clients[c.userID] = c
			h.mu.Unlock()
			h.log.Debug("chat: registered", "user", c.userID)
			select {
			case h.presenceC <- presenceChange{userID: c.userID, online: true}:
			default:
			}
		case c := <-h.unregister:
			h.mu.Lock()
			if cur, ok := h.clients[c.userID]; ok && cur == c {
				delete(h.clients, c.userID)
			}
			h.mu.Unlock()
			c.shutdown()
			h.log.Debug("chat: unregistered", "user", c.userID)
			select {
			case h.presenceC <- presenceChange{userID: c.userID, online: false}:
			default:
			}
		}
	}
}

// deliver queues msg for targetID. Returns false if targetID is not connected.
func (h *Hub) deliver(targetID string, msg []byte) bool {
	h.mu.RLock()
	c, ok := h.clients[targetID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	return c.tryDeliver(msg)
}
