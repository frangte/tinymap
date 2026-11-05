package hub

import (
	"errors"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"

	"saigon-noir/server/internal/protocol"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = pongWait * 9 / 10
)

// Per-connection limits: states arrive at 10 Hz, so 20 Hz leaves room for jitter; chat about one line a second.
var (
	stateRate    = rate.Limit(20)
	stateBurst   = 20
	chatRate     = rate.Every(700 * time.Millisecond)
	chatBurst    = 3
	profileRate  = rate.Every(250 * time.Millisecond)
	profileBurst = 4
)

// Client is one WebSocket connection. Its read loop runs on the HTTP handler goroutine, its write loop on its own.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	// closed is owned by the hub goroutine: set when send has been closed
	closed bool
}

// Serve runs a connection until it closes. Call it from the HTTP handler after the upgrade.
func (h *Hub) Serve(conn *websocket.Conn) {
	c := &Client{hub: h, conn: conn, send: make(chan []byte, h.cfg.SendBuffer)}
	go c.writeLoop()
	c.readLoop()
}

// readLoop never closes the socket itself while the hub runs: every way out ends in an evLeave, the hub then
// closes the send queue, and writeLoop closes the socket after flushing what is queued (such as an error code).
func (c *Client) readLoop() {
	h := c.hub
	c.conn.SetReadLimit(protocol.MaxMessageBytes)
	leave := func(code string) {
		if !h.submit(event{kind: evLeave, c: c, code: code}) {
			c.conn.Close() // hub stopped: nobody will close the queue
		}
	}

	// the first frame must be a valid hello, and soon
	_ = c.conn.SetReadDeadline(time.Now().Add(h.cfg.HelloTimeout))
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		leave("")
		return
	}
	hello, err := protocol.Decode(data)
	if err != nil || hello.T != protocol.TypeHello {
		leave(protocol.ErrBadHello)
		return
	}
	if !h.submit(event{kind: evJoin, c: c, msg: hello}) {
		c.conn.Close()
		return
	}

	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	states := rate.NewLimiter(stateRate, stateBurst)
	chats := rate.NewLimiter(chatRate, chatBurst)
	profiles := rate.NewLimiter(profileRate, profileBurst)
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) && !errors.Is(err, websocket.ErrReadLimit) {
				h.log.Debug("read", "err", err)
			}
			leave("")
			return
		}
		msg, err := protocol.Decode(data)
		if err != nil {
			continue // a malformed or out-of-range message is dropped, the connection stays
		}
		kind := evMsg
		switch msg.T {
		case protocol.TypeState:
			if !states.Allow() {
				continue
			}
		case protocol.TypeProfile:
			if !profiles.Allow() {
				continue
			}
		case protocol.TypeChat:
			if !chats.Allow() {
				kind = evRateLimited
			}
		case protocol.TypeHello:
			continue
		}
		if !h.submit(event{kind: kind, c: c, msg: msg}) {
			c.conn.Close()
			return
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
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
