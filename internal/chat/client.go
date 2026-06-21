package chat

import (
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10 // must be less than pongWait
	maxMsgSize = 4 << 10             // 4 KB
)

// Client is a middleman between one user's WebSocket connection and the Hub.
type Client struct {
	hub    *Hub
	userID string
	conn   *websocket.Conn
	outbox chan []byte
	done   atomic.Bool
}

func newClient(hub *Hub, userID string, conn *websocket.Conn) *Client {
	return &Client{
		hub:    hub,
		userID: userID,
		conn:   conn,
		outbox: make(chan []byte, 256),
	}
}

// tryDeliver queues msg. Returns false when the outbox is full or the client is shut down.
func (c *Client) tryDeliver(msg []byte) bool {
	if c.done.Load() {
		return false
	}
	select {
	case c.outbox <- msg:
		return true
	default:
		return false
	}
}

// shutdown closes the outbox channel once, signalling writePump to exit.
func (c *Client) shutdown() {
	if c.done.CompareAndSwap(false, true) {
		close(c.outbox)
	}
}

// readPump reads frames from the WebSocket and hands them to the service.
// It runs in its own goroutine; when it exits it unregisters the client.
func (c *Client) readPump(svc *Service) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMsgSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		svc.handleInbound(c, raw)
	}
}

// writePump drains outbox to the WebSocket and sends periodic pings.
// It runs in its own goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.outbox:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Outbox closed (shutdown or displacement).
				_ = c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
