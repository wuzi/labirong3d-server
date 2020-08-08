package network

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"

	"labirong3d.com/server/entity"
	"labirong3d.com/server/util"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline         = []byte{'\n'}
	space           = []byte{' '}
	playerJoinCount = 0
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The player data
	player *entity.Player

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		// Tell clients that a new player joined.
		data := struct {
			Player *entity.Player `json:"player"`
		}{Player: c.player}

		e := Event{"playerQuit", data}
		c.hub.broadcast <- e

		// Close connection.
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		var event Event
		json.Unmarshal(data, &event)
		c.processEvent(event)
	}
}

// processEvent executes instructions based on the event name
func (c *Client) processEvent(event Event) {
	if event.Name == "movePlayer" {
		// Update player's position
		var localPlayer struct {
			Position         util.Vector3 `json:"position"`
			Rotation         util.Vector3 `json:"rotation"`
			CurrentAnimation string       `json:"currentAnimation"`
		}
		err := mapstructure.Decode(event.Data, &localPlayer)
		if err != nil {
			log.Printf("error: %v", err)
			return
		}
		c.player.Position = localPlayer.Position
		c.player.Rotation = localPlayer.Rotation
		c.player.CurrentAnimation = localPlayer.CurrentAnimation

		// Broadcast the new position to all clients
		data := struct {
			Players []*entity.Player `json:"players"`
		}{[]*entity.Player{c.player}}
		e := Event{"update", data}
		event, _ := json.Marshal(e)
		for client, active := range c.hub.clients {
			if active == false || client.player.ID == c.player.ID {
				continue
			}
			client.send <- event
		}
	} else if event.Name == "syncWorld" {
		data := struct {
			Players []*entity.Player `json:"players"`
			Grid    [][]int          `json:"grid"`
		}{[]*entity.Player{}, c.hub.grid}

		for client, active := range c.hub.clients {
			if active == false || client.player.ID == c.player.ID {
				continue
			}
			data.Players = append(data.Players, client.player)
		}

		e := Event{"syncWorld", data}
		event, _ := json.Marshal(e)
		c.send <- event
	} else if event.Name == "chatMessage" {
		// Receive message
		var incomingData struct {
			Message string `json:"message"`
		}

		err := mapstructure.Decode(event.Data, &incomingData)
		if err != nil {
			log.Printf("error: %v", err)
			return
		}

		// Broadcast message to all clients
		outgoingData := struct {
			Player  *entity.Player `json:"player"`
			Message string         `json:"message"`
		}{c.player, incomingData.Message}

		e := Event{"chatMessage", outgoingData}
		event, _ := json.Marshal(e)

		for client, active := range c.hub.clients {
			if active == false {
				continue
			}
			client.send <- event
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWs handles websocket requests from the peer.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := upgrader.Upgrade(w, r, nil)
	vars := r.URL.Query()
	if err != nil {
		log.Println(err)
		return
	}

	playerJoinCount++
	player := &entity.Player{
		ID:               playerJoinCount,
		Name:             vars.Get("name"),
		Color:            vars.Get("color"),
		Position:         util.Vector3{},
		Rotation:         util.Vector3{},
		CurrentAnimation: "Idle",
	}

	// Tell clients that a new player joined.
	data := struct {
		Player *entity.Player `json:"player"`
	}{player}
	e := Event{"playerJoin", data}
	hub.broadcast <- e

	// Create client for the new connection
	client := &Client{hub: hub, player: player, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
