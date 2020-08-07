package network

// Event is the struct sent and received from the clients
type Event struct {
	Name string      `json:"name"`
	Data interface{} `json:"data"`
}
