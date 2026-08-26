package ws

import (
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingInterval   = (pongWait * 9) / 10
	sendBufferSize = 16
)

// Client is one connected WebSocket subscriber, scoped to a single
// department. It has no knowledge of Redis or the domain; the hub hands
// it pre-serialized JSON to write.
type Client struct {
	hub          *Hub
	conn         *websocket.Conn
	departmentID string
	sendCh       chan []byte
	log          *slog.Logger
}

func NewClient(hub *Hub, conn *websocket.Conn, departmentID string, log *slog.Logger) *Client {
	return &Client{
		hub:          hub,
		conn:         conn,
		departmentID: departmentID,
		sendCh:       make(chan []byte, sendBufferSize),
		log:          log,
	}
}

func (c *Client) send(message []byte) {
	select {
	case c.sendCh <- message:
	default:
		// Slow consumer: drop the message rather than block the hub or
		// grow memory unboundedly. The client's next snapshot fetch (or
		// the next event) will bring it back in sync.
		c.log.Warn("ws: client send buffer full, dropping message", "department_id", c.departmentID)
	}
}

// Run starts the read and write pumps and blocks until the connection
// closes. Call it in its own goroutine per connection.
func (c *Client) Run() {
	c.hub.Register(c)
	defer c.hub.Unregister(c)

	done := make(chan struct{})
	go c.writePump(done)
	c.readPump()
	close(done)
}

// readPump does nothing with inbound messages beyond keeping the
// connection's read deadline alive via pong handling; this is a
// server-to-client broadcast channel, not a command channel.
func (c *Client) readPump() {
	defer c.conn.Close()
	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *Client) writePump(done <-chan struct{}) {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-done:
			return
		case message, ok := <-c.sendCh:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
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
