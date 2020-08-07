package network

import (
	"encoding/json"

	"labirong3d.com/server/util"
)

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	// Auto generated grid.
	grid [][]int

	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan Event

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		grid:       util.MakeGrid(16, 16),
		broadcast:  make(chan Event),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// Run serve the hub to listen for new messages.
func (h *Hub) Run() {
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
