package models

import "time"

// Position represents a 3D position in the virtual map
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Obstacle represents an obstacle in the map
type Obstacle struct {
	ID       string   `json:"id"`
	Position Position `json:"position"`
	Radius   float64  `json:"radius"`
	Type     string   `json:"type"` // "circle", "square", "wall"
}

// Goal represents a target position in the map
type Goal struct {
	ID       string   `json:"id"`
	Position Position `json:"position"`
	Status   string   `json:"status"` // "pending", "active", "reached", "failed"
	Radius   float64  `json:"radius"`
}

// MapGrid represents the virtual map structure
type MapGrid struct {
	ID        string     `json:"id"`
	Width     float64    `json:"width"`
	Height    float64    `json:"height"`
	CellSize  float64    `json:"cell_size"`
	Obstacles []Obstacle `json:"obstacles"`
	Goals     []Goal     `json:"goals"`
	StartPos  Position   `json:"start_position"`
	CreatedAt time.Time  `json:"created_at"`
}

// MapGridMessage is the WebSocket message for map broadcasting
type MapGridMessage struct {
	MapID     string     `json:"map_id"`
	Width     float64    `json:"width"`
	Height    float64    `json:"height"`
	CellSize  float64    `json:"cell_size"`
	Obstacles []Obstacle `json:"obstacles"`
	Goals     []Goal     `json:"goals"`
	StartPos  Position   `json:"start_position"`
}

// AGVCommandMessage represents a command sent to AGV
type AGVCommandMessage struct {
	AGVID     string   `json:"agv_id"`
	Command   string   `json:"command"` // "move_to", "stop", "reset"
	TargetPos Position `json:"target_position"`
	Timestamp int64    `json:"timestamp"`
}
