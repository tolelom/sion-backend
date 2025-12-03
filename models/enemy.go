package models

import "time"

// ========================================
// 적 타입 상수
// ========================================
const (
	EnemyTypeAhri     = "ahri"     // 기본 타겟 (아리)
	EnemyTypeObstacle = "obstacle" // 장애물 (회피 대상)
)

// 적 상태 상수
const (
	EnemyStateAlive    = "alive"    // 생존
	EnemyStateDefeated = "defeated" // 처치됨
	EnemyStateEscaped  = "escaped"  // 도주함
)

// ========================================
// 적 정보
// ========================================
type Enemy struct {
	// 기본 정보
	ID    string `json:"id"`    // 고유 ID
	Type  string `json:"type"`  // "ahri" | "obstacle"
	Name  string `json:"name"`  // 이름
	State string `json:"state"` // 상태

	// 위치 정보
	Position   PositionData `json:"position"`     // 현재 위치
	Distance   float64      `json:"distance"`     // AGV로부터의 거리 (m)
	AngleToAGV float64      `json:"angle_to_agv"` // AGV 기준 각도 (라디안)

	// 상태 정보
	HP    int     `json:"hp"`     // 체력 (0-100)
	MaxHP int     `json:"max_hp"` // 최대 체력
	Speed float64 `json:"speed"`  // 이동 속도 (m/s)

	// 탐지 정보
	FirstDetected  time.Time `json:"first_detected"`  // 처음 발견 시각
	LastSeen       time.Time `json:"last_seen"`       // 마지막 발견 시각
	DetectionCount int       `json:"detection_count"` // 총 탐지 횟수

	// 우선순위 계산용
	Priority    int    `json:"priority"`     // 계산된 우선순위 (높을수록 우선)
	ThreatLevel string `json:"threat_level"` // "low" | "medium" | "high"
}

// ========================================
// 타겟 선택 결과
// ========================================
type TargetSelection struct {
	SelectedEnemy *Enemy    `json:"selected_enemy"` // 선택된 적
	Reason        string    `json:"reason"`         // 선택 이유
	Alternatives  []Enemy   `json:"alternatives"`   // 대안 타겟들
	Timestamp     time.Time `json:"timestamp"`      // 선택 시각
}

// ========================================
// 적 그룹 정보 (여러 적 관리)
// ========================================
type EnemyGroup struct {
	Enemies       []Enemy   `json:"enemies"`        // 적 리스트
	TotalCount    int       `json:"total_count"`    // 총 적 수
	ActiveCount   int       `json:"active_count"`   // 활성 적 수
	DefeatedCount int       `json:"defeated_count"` // 처치된 적 수
	LastUpdate    time.Time `json:"last_update"`    // 마지막 업데이트
}

// ========================================
// 공격 결과
// ========================================
type AttackResult struct {
	EnemyID     string    `json:"enemy_id"`     // 공격한 적 ID
	Damage      int       `json:"damage"`       // 입힌 피해량
	RemainingHP int       `json:"remaining_hp"` // 남은 체력
	Defeated    bool      `json:"defeated"`     // 처치 여부
	Timestamp   time.Time `json:"timestamp"`    // 공격 시각
}

// ========================================
// 우선순위 계산 설정
// ========================================
type PriorityConfig struct {
	DistanceWeight float64 `json:"distance_weight"` // 거리 가중치 (0-1)
	HPWeight       float64 `json:"hp_weight"`       // 체력 가중치 (0-1)
	ThreatWeight   float64 `json:"threat_weight"`   // 위협도 가중치 (0-1)
}

// ========================================
// 타겟 발견 이벤트
// ========================================
type TargetFoundEvent struct {
	Enemy           Enemy     `json:"enemy"`            // 발견된 적
	DetectionMethod string    `json:"detection_method"` // "camera" | "sensor"
	Confidence      float64   `json:"confidence"`       // 탐지 신뢰도 (0-1)
	Timestamp       time.Time `json:"timestamp"`        // 발견 시각
}
