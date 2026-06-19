package websocket

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ChannelWSMessage struct {
	Type      string      `json:"type"`
	ChannelID uint        `json:"channel_id,omitempty"`
	Content   string      `json:"content,omitempty"`
	UserID    uint        `json:"user_id,omitempty"`
	UserName  string      `json:"user_name,omitempty"`
	TempID    string      `json:"temp_id,omitempty"`
	Timestamp time.Time   `json:"timestamp,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

var ChannelUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkWSOrigin,
}

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	// writeWait bounds every socket write so a wedged TCP connection can never
	// block its writer goroutine indefinitely.
	writeWait = 10 * time.Second
	// sendBuffer is the per-connection outbound queue depth. A client that falls
	// this far behind (slow/stalled socket) is dropped instead of stalling the hub.
	sendBuffer = 64
	// broadcastBuffer decouples HTTP handlers (and the service broadcaster) from
	// the hub loop so a momentary hub stall does not block request handling.
	broadcastBuffer = 256
)

// channelClient is one live WebSocket connection. Each client owns a single
// writer goroutine (writePump); all writes to conn happen there and nowhere
// else, so we never need a per-connection write mutex and the hub loop never
// performs network I/O under h.mu.
type channelClient struct {
	conn   *websocket.Conn
	userID uint
	send   chan ChannelWSMessage
	// done is closed exactly once to signal that this client is being torn down.
	// We deliberately never close c.send: closing it would race with concurrent
	// non-blocking sends from broadcast/ping (a "send on a closed channel" case
	// in a select is "ready" and the runtime may pick it over default, so close()
	// happening anywhere fully outside the send's lock can still panic). Instead,
	// every sender selects on <-c.done as well, so a closing client is observed
	// via done and is never sent to. Closing done is always safe because NOBODY
	// ever sends to done — only close it. The unsent c.send is reclaimed by GC.
	done chan struct{}
	// closeOnce guarantees done is closed exactly once.
	closeOnce sync.Once
}

// closeSend signals teardown by closing c.done exactly once. It is safe to call
// from any goroutine and any number of times: closing done can never panic
// because no goroutine ever sends to done, only closes it. writePump observes
// the closed done and returns; enqueue observes it and stops sending.
func (c *channelClient) closeSend() {
	c.closeOnce.Do(func() { close(c.done) })
}

type channelRegistration struct {
	conn   *websocket.Conn
	userID uint
}

type ChannelHub struct {
	clients        map[*websocket.Conn]*channelClient
	broadcast      chan ChannelWSMessage
	register       chan channelRegistration
	unregister     chan *websocket.Conn
	mu             sync.RWMutex
	MemberResolver func(channelID uint) map[uint]bool
}

func NewChannelHub() *ChannelHub {
	return &ChannelHub{
		clients:    make(map[*websocket.Conn]*channelClient),
		broadcast:  make(chan ChannelWSMessage, broadcastBuffer),
		register:   make(chan channelRegistration),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *ChannelHub) Run() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case reg := <-h.register:
			client := &channelClient{
				conn:   reg.conn,
				userID: reg.userID,
				send:   make(chan ChannelWSMessage, sendBuffer),
				done:   make(chan struct{}),
			}
			h.mu.Lock()
			h.clients[reg.conn] = client
			h.mu.Unlock()
			// One writer goroutine per connection: it owns every write to conn
			// (data frames + pings) and exits when send is closed or a write fails.
			go h.writePump(client)

		case conn := <-h.unregister:
			h.removeClient(conn)

		case message := <-h.broadcast:
			h.dispatch(message)

		case <-ticker.C:
			// Enqueue a ping onto each client's send channel; the actual
			// WriteMessage(ping) happens in writePump, off the hub loop and h.mu.
			h.mu.RLock()
			clients := make([]*channelClient, 0, len(h.clients))
			for _, c := range h.clients {
				clients = append(clients, c)
			}
			h.mu.RUnlock()
			for _, c := range clients {
				h.enqueue(c, ChannelWSMessage{Type: "__ping"})
			}
		}
	}
}

// dispatch routes a broadcast to the connections whose user is a member of the
// target channel. Under h.mu we only read the client set and do non-blocking
// channel sends — never network I/O — so a slow client cannot freeze the hub.
func (h *ChannelHub) dispatch(message ChannelWSMessage) {
	if message.ChannelID == 0 {
		return
	}

	var members map[uint]bool
	if h.MemberResolver != nil {
		members = h.MemberResolver(message.ChannelID)
	}

	h.mu.RLock()
	targets := make([]*channelClient, 0, len(h.clients))
	for _, c := range h.clients {
		// Intentional fail-closed policy: with no MemberResolver (members == nil)
		// we cannot authorize anyone, so we deliver to NOBODY rather than risk
		// leaking a channel's traffic to non-members. In production the resolver
		// is always wired (see routes/deps.go), so this only bites in a
		// misconfigured/test setup — where dropping is the safe default.
		if members == nil || !members[c.userID] {
			continue
		}
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		h.enqueue(c, message)
	}
}

// enqueue does a non-blocking send onto the client's outbound queue. If the
// client is already tearing down (done closed), we observe that case instead of
// touching send. If the queue is full the client is too slow (TCP backpressure
// / stalled socket), so we drop and unregister it rather than block the hub.
// NEVER blocks, and can NEVER panic: c.send is never closed, so there is no
// "send on closed channel" hazard; the <-c.done case absorbs closing clients.
func (h *ChannelHub) enqueue(c *channelClient, message ChannelWSMessage) {
	select {
	case c.send <- message:
	case <-c.done:
		// Client is already being torn down; drop silently.
	default:
		// Slow client: tear it down asynchronously so we don't recurse into the
		// hub loop while holding nothing. removeClient closes done (once), which
		// makes writePump exit.
		go h.removeClient(c.conn)
	}
}

// removeClient deregisters a connection and signals teardown by closing its
// done channel exactly once. c.send is never closed, so non-blocking sends
// elsewhere can never panic on a closed channel.
func (h *ChannelHub) removeClient(conn *websocket.Conn) {
	h.mu.Lock()
	client, ok := h.clients[conn]
	if ok {
		delete(h.clients, conn)
	}
	h.mu.Unlock()

	if ok {
		client.closeSend()
	}
}

// writePump is the sole writer for a connection. It drains c.send and writes
// each message (or a ping for the internal "__ping" sentinel) with a write
// deadline. It exits when done is closed (unregister) or any write fails. The
// defer'd conn.Close() unblocks the read-loop in HandleConnection. Because
// c.send is never closed, writePump must watch c.done rather than rely on a
// closed-channel range exit.
func (h *ChannelHub) writePump(c *channelClient) {
	defer c.conn.Close()
	for {
		select {
		case msg := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if msg.Type == "__ping" {
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
				continue
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

func (h *ChannelHub) HandleConnection(w http.ResponseWriter, r *http.Request, userID uint) {
	conn, err := ChannelUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	h.register <- channelRegistration{conn: conn, userID: userID}

	// Inbound frames are typing indicators only — keep them tiny.
	conn.SetReadLimit(1024)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go func() {
		// On any read error/close, deregister via the hub (which closes send and
		// stops the writer). conn.Close() is handled by writePump's defer.
		defer func() {
			h.unregister <- conn
		}()

		var lastTyping time.Time
		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var in ChannelWSMessage
			if err := json.Unmarshal(msgBytes, &in); err != nil {
				continue
			}

			// Clients may only emit typing indicators. Every other event type
			// (message, message_edited, message_deleted, reactions, ...) is
			// server-authored by the HTTP handlers after persisting — relaying
			// them here would let a client forge UI events for other members.
			if in.Type != "typing" || in.ChannelID == 0 {
				continue
			}
			// Rate-limit typing to one event per second per connection.
			if time.Since(lastTyping) < time.Second {
				continue
			}
			// Only members of the channel can signal typing in it.
			if h.MemberResolver != nil && !h.MemberResolver(in.ChannelID)[userID] {
				continue
			}
			lastTyping = time.Now()

			h.broadcast <- ChannelWSMessage{
				Type:      "typing",
				ChannelID: in.ChannelID,
				UserID:    userID,
				UserName:  in.UserName,
				Timestamp: time.Now(),
			}
		}
	}()
}

func (h *ChannelHub) Broadcast(msg ChannelWSMessage) {
	h.broadcast <- msg
}
