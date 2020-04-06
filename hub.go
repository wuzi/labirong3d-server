package main

import (
	"encoding/json"
	"time"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan Event

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Ticker to update clients about everything
	ticker *time.Ticker
}

// newHub creates a new Hub.
func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan Event),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		ticker:     time.NewTicker(1 * time.Second),
	}
}

// tick starts the hub ticker.
func (h *Hub) tick() {
	for range h.ticker.C {
		var players []*Player
		for client, active := range h.clients {
			if active == false {
				continue
			}
			players = append(players, client.player)
		}
		e := Event{"update", players}
		h.broadcast <- e
	}
}

// Run serve the hub to listen for new messages.
func (h *Hub) run() {
	go h.tick()
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case e := <-h.broadcast:
			event, _ := json.Marshal(e)
			for client := range h.clients {
				select {
				case client.send <- event:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
