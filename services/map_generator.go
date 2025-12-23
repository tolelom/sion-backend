package services

import (
	"fmt"
	"math"
	"math/rand"
	"sion-backend/models"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MapGenerator handles virtual map generation and management
type MapGenerator struct {
	mu           sync.RWMutex
	activeMap    *models.MapGrid
	generationMu sync.Mutex
	rng          *rand.Rand
}

// NewMapGenerator creates a new MapGenerator instance
func NewMapGenerator() *MapGenerator {
	return &MapGenerator{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateMap creates a new virtual map with random obstacles
func (mg *MapGenerator) GenerateMap(width, height, cellSize float64) *models.MapGrid {
	mg.generationMu.Lock()
	defer mg.generationMu.Unlock()

	mapGrid := &models.MapGrid{
		ID:        uuid.New().String(),
		Width:     width,
		Height:    height,
		CellSize:  cellSize,
		Obstacles: mg.generateObstacles(width, height, 5), // 5개의 랜덤 장애물
		Goals:     []models.Goal{},
		StartPos: models.Position{
			X: width / 2,
			Y: height / 2,
			Z: 0,
		},
		CreatedAt: time.Now(),
	}

	mg.mu.Lock()
	mg.activeMap = mapGrid
	mg.mu.Unlock()

	return mapGrid
}

// generateObstacles creates random obstacles in the map
func (mg *MapGenerator) generateObstacles(width, height float64, count int) []models.Obstacle {
	obstacles := make([]models.Obstacle, 0, count)

	// 경계에서 안전한 여백 (10%)
	margin := 0.1
	minX := width * margin
	maxX := width * (1 - margin)
	minY := height * margin
	maxY := height * (1 - margin)

	for i := 0; i < count; i++ {
		obstacles = append(obstacles, models.Obstacle{
			ID: fmt.Sprintf("obstacle-%d", i+1),
			Position: models.Position{
				X: minX + mg.rng.Float64()*(maxX-minX),
				Y: minY + mg.rng.Float64()*(maxY-minY),
				Z: 0,
			},
			Radius: 0.5 + mg.rng.Float64()*1.0, // 반경 0.5~1.5m
			Type:   "circle",
		})
	}

	return obstacles
}

// GetActiveMap returns the current active map
func (mg *MapGenerator) GetActiveMap() *models.MapGrid {
	mg.mu.RLock()
	defer mg.mu.RUnlock()
	return mg.activeMap
}

// AddGoal adds a new goal to the active map
func (mg *MapGenerator) AddGoal(position models.Position, radius float64) (*models.Goal, error) {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	if mg.activeMap == nil {
		return nil, fmt.Errorf("no active map")
	}

	goal := models.Goal{
		ID:       uuid.New().String(),
		Position: position,
		Status:   "pending",
		Radius:   radius,
	}

	mg.activeMap.Goals = append(mg.activeMap.Goals, goal)
	return &goal, nil
}

// UpdateGoal updates an existing goal's position or status
func (mg *MapGenerator) UpdateGoal(goalID string, position models.Position) error {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	if mg.activeMap == nil {
		return fmt.Errorf("no active map")
	}

	for i := range mg.activeMap.Goals {
		if mg.activeMap.Goals[i].ID == goalID {
			mg.activeMap.Goals[i].Position = position
			return nil
		}
	}

	return fmt.Errorf("goal not found: %s", goalID)
}

// SetGoalStatus updates a goal's status
func (mg *MapGenerator) SetGoalStatus(goalID string, status string) error {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	if mg.activeMap == nil {
		return fmt.Errorf("no active map")
	}

	for i := range mg.activeMap.Goals {
		if mg.activeMap.Goals[i].ID == goalID {
			mg.activeMap.Goals[i].Status = status
			return nil
		}
	}

	return fmt.Errorf("goal not found: %s", goalID)
}

// IsPositionValid checks if a position is valid (not inside obstacles)
func (mg *MapGenerator) IsPositionValid(pos models.Position) bool {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	if mg.activeMap == nil {
		return false
	}

	// 경계 체크
	if pos.X < 0 || pos.X > mg.activeMap.Width || pos.Y < 0 || pos.Y > mg.activeMap.Height {
		return false
	}

	// 장애물과 충돌 체크
	for _, obstacle := range mg.activeMap.Obstacles {
		dist := math.Sqrt(
			math.Pow(pos.X-obstacle.Position.X, 2) +
				math.Pow(pos.Y-obstacle.Position.Y, 2),
		)
		if dist < obstacle.Radius {
			return false
		}
	}

	return true
}

// GetMapGridMessage converts MapGrid to WebSocket message format
func (mg *MapGenerator) GetMapGridMessage() *models.MapGridMessage {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	if mg.activeMap == nil {
		return nil
	}

	return &models.MapGridMessage{
		MapID:     mg.activeMap.ID,
		Width:     mg.activeMap.Width,
		Height:    mg.activeMap.Height,
		CellSize:  mg.activeMap.CellSize,
		Obstacles: mg.activeMap.Obstacles,
		Goals:     mg.activeMap.Goals,
		StartPos:  mg.activeMap.StartPos,
	}
}

// ClearMap removes the current active map
func (mg *MapGenerator) ClearMap() {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	mg.activeMap = nil
}
