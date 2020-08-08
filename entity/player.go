package entity

import (
	"labirong3d.com/server/util"
)

// Player is the struct with the player's data
type Player struct {
	ID               int          `json:"id"`
	Name             string       `json:"name"`
	Color            string       `json:"color"`
	Position         util.Vector3 `json:"position"`
	Rotation         util.Vector3 `json:"rotation"`
	CurrentAnimation string       `json:"currentAnimation"`
}
