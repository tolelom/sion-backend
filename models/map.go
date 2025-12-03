package models

import "time"

// ========================================
// 맵 타입 상수
// ========================================
const (
	CellEmpty    = 0 // 빈 공간
	CellObstacle = 1 // 장애물
	CellAGV      = 2 // AGV 위치
	CellEnemy    = 3 // 적 위치
	CellPath     = 4 // 경로
)

// ========================================
// 가상 맵 정보
// ========================================
type Map struct {
	// 기본 정보
	ID       string  `json:"id"`        // 맵 ID
	Name     string  `json:"name"`      // 맵 이름
	Width    int     `json:"width"`     // 맵 너비 (그리드 수)
	Height   int     `json:"height"`    // 맵 높이 (그리드 수)
	CellSize float64 `json:"cell_size"` // 각 그리드 셀 크기 (미터)

	// 맵 데이터
	Grid [][]int `json:"grid"` // 2D 그리드 (0: 빈공간, 1: 장애물)

	// 메타 데이터
	CreatedAt time.Time `json:"created_at"` // 생성 시각
	UpdatedAt time.Time `json:"updated_at"` // 마지막 업데이트
}

// ========================================
// 맵 좌표 변환 유틸리티
// ========================================

// 실제 좌표를 그리드 좌표로 변환
type GridCoordinate struct {
	Row int `json:"row"` // 행 (그리드 Y)
	Col int `json:"col"` // 열 (그리드 X)
}

// 실제 물리적 좌표
type RealCoordinate struct {
	X float64 `json:"x"` // 실제 X 좌표 (미터)
	Y float64 `json:"y"` // 실제 Y 좌표 (미터)
}

// ========================================
// 맵 업데이트 이벤트
// ========================================
type MapUpdate struct {
	UpdateType    string           `json:"update_type"`    // "obstacle_added" | "obstacle_removed" | "full_update"
	AffectedCells []GridCoordinate `json:"affected_cells"` // 변경된 셀
	NewGrid       [][]int          `json:"new_grid"`       // 전체 그리드 (필요시)
	Timestamp     time.Time        `json:"timestamp"`      // 업데이트 시각
}

// ========================================
// 장애물 정보
// ========================================
type Obstacle struct {
	ID         string         `json:"id"`          // 장애물 ID
	Type       string         `json:"type"`        // "static" | "dynamic"
	Position   GridCoordinate `json:"position"`    // 그리드 위치
	Size       int            `json:"size"`        // 크기 (그리드 셀 수)
	DetectedAt time.Time      `json:"detected_at"` // 발견 시각
}

// ========================================
// 경로 계획 요청
// ========================================
type PathPlanningRequest struct {
	Start        PositionData `json:"start"`         // 시작점
	Goal         PositionData `json:"goal"`          // 목표점
	Algorithm    string       `json:"algorithm"`     // "a_star" | "dijkstra"
	AvoidEnemies bool         `json:"avoid_enemies"` // 적 회피 여부
	MaxDistance  float64      `json:"max_distance"`  // 최대 거리 제한
}

// ========================================
// 경로 계획 응답
// ========================================
type PathPlanningResponse struct {
	Path          []PositionData `json:"path"`           // 경로 포인트
	Length        float64        `json:"length"`         // 총 경로 길이
	EstimatedTime float64        `json:"estimated_time"` // 예상 소요 시간 (초)
	Algorithm     string         `json:"algorithm"`      // 사용된 알고리즘
	Success       bool           `json:"success"`        // 성공 여부
	Message       string         `json:"message"`        // 에러/경고 메시지
	CreatedAt     time.Time      `json:"created_at"`     // 생성 시각
}

// ========================================
// 맵 영역 (관심 영역)
// ========================================
type MapRegion struct {
	ID          string         `json:"id"`           // 영역 ID
	Name        string         `json:"name"`         // 영역 이름
	TopLeft     GridCoordinate `json:"top_left"`     // 좌상단 좌표
	BottomRight GridCoordinate `json:"bottom_right"` // 우하단 좌표
	Description string         `json:"description"`  // 설명
}

// ========================================
// 실시간 맵 상태
// ========================================
type MapState struct {
	Map         Map          `json:"map"`          // 기본 맵
	AGVPosition PositionData `json:"agv_position"` // AGV 현재 위치
	Enemies     []Enemy      `json:"enemies"`      // 모든 적
	Obstacles   []Obstacle   `json:"obstacles"`    // 모든 장애물
	CurrentPath *PathData    `json:"current_path"` // 현재 경로
	Timestamp   time.Time    `json:"timestamp"`    // 스냅샷 시각
}
