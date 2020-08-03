package main

// Player is the struct with the player's data
type Player struct {
	ID               int     `json:"id"`
	Position         Vector3 `json:"position"`
	Rotation         Vector3 `json:"rotation"`
	CurrentAnimation string  `json:"currentAnimation"`
}
