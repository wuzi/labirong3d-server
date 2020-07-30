package main

import (
	"math/rand"
	"time"
)

var grid [][]int
var attemptsLeft int
var generating bool

// makeGrid creates a new grid
func makeGrid(sizeX, sizeY int) [][]int {
	rand.Seed(time.Now().UTC().UnixNano())

	attemptsLeft = 5
	generating = true

	// initialize grid
	grid = make([][]int, sizeX)
	for i := range grid {
		grid[i] = make([]int, sizeY)
	}

	// fill the grid with walls
	for x := 0; x < sizeX; x++ {
		for y := 0; y < sizeY; y++ {
			grid[x][y] = 1
		}
	}

	// create a starting point in the middle of first column to generate from
	grid[len(grid)/2][0] = 0

	for generating {
		generate()
	}

	// close entrance
	grid[len(grid)/2][0] = 1
	return grid
}

// generates a new map
func generate() {
	pCount := 0

	for x := 0; x < len(grid); x++ {
		for y := 0; y < len(grid[0]); y++ {
			if getTile(x, y) == 0 {
				pCount += makePassage(x, y, -1, 0)
				pCount += makePassage(x, y, 1, 0)
				pCount += makePassage(x, y, 0, -1)
				pCount += makePassage(x, y, 0, 1)
			}
		}
	}

	if pCount == 0 {
		attemptsLeft--
		if attemptsLeft < 0 {
			possibleExits := []int{}
			for x := 0; x < len(grid); x++ {
				if getTile(x, len(grid[0])-2) == 0 {
					possibleExits = append(possibleExits, x)
				}
			}

			// create a random exit
			x := possibleExits[rand.Intn(len(possibleExits))]
			setTile(x, len(grid[0])-1, 0)

			generating = false
		}
	}
}

// makePassage checks around a coordinate if it's all walls
// and randomly creates a passage
func makePassage(x, y, i, j int) int {
	if getTile(x+i, y+j) == 1 &&
		getTile(x+i+j, y+j+i) == 1 &&
		getTile(x+i-j, y+j-i) == 1 {
		if getTile(x+i+i, y+j+j) == 1 &&
			getTile(x+i+i+j, y+j+j+i) == 1 &&
			getTile(x+i+i-j, y+j+j-i) == 1 {
			if rand.Float32() > 0.5 {
				setTile(x+i, y+j, 0)
				return 1
			}
		}
	}
	return 0
}

// getTile gets the type of a tile in a x and y coordinate
func getTile(x, y int) int {
	if x >= 0 && y >= 0 && x < len(grid) && y < len(grid[0]) {
		return grid[x][y]
	}
	return 0
}

// setTile sets the type of a tile of a x and y coordinate
func setTile(x, y int, tile int) {
	if x >= 0 && y >= 0 && x < len(grid) && y < len(grid[0]) {
		grid[x][y] = tile
	}
}
