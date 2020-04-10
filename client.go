package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
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
	player *Player

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
			ID int `json:"id"`
		}{ID: c.player.ID}

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
			Position Vector3 `json:"position"`
			Rotation Vector3 `json:"rotation"`
		}
		err := mapstructure.Decode(event.Data, &localPlayer)
		if err != nil {
			log.Printf("error: %v", err)
			return
		}
		c.player.Position = localPlayer.Position
		c.player.Rotation = localPlayer.Rotation

		// Broadcast the new position to all clients
		data := struct {
			Players []*Player `json:"players"`
		}{[]*Player{c.player}}
		e := Event{"update", data}
		event, _ := json.Marshal(e)
		for client, active := range c.hub.clients {
			if active == false || client.player.ID == c.player.ID {
				continue
			}
			client.send <- event
		}
	} else if event.Name == "getConnectedPlayers" {
		data := struct {
			Players []*Player `json:"players"`
		}{[]*Player{}}

		for client, active := range c.hub.clients {
			if active == false || client.player.ID == c.player.ID {
				continue
			}
			data.Players = append(data.Players, client.player)
		}

		e := Event{"getConnectedPlayers", data}
		event, _ := json.Marshal(e)
		c.send <- event
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

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	playerJoinCount++

	// Tell clients that a new player joined.
	data := struct {
		ID int `json:"id"`
	}{ID: playerJoinCount}

	e := Event{"playerJoin", data}
	hub.broadcast <- e

	// Create client for the new connection
	player := &Player{ID: playerJoinCount, Position: Vector3{}, Rotation: Vector3{}}
	client := &Client{hub: hub, player: player, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
