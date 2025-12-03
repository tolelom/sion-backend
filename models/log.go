package models

import (
	"time"
)

// AGVLog - AGV 행동 로그
type AGVLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	EventType   string    `json:"event_type"` // "position", "status", "target_found", "command", etc.
	MessageType string    `json:"message_type"`

	// AGV 상태
	AGVID         string  `json:"agv_id"`
	PositionX     float64 `json:"position_x"`
	PositionY     float64 `json:"position_y"`
	PositionAngle float64 `json:"position_angle"`
	Speed         float64 `json:"speed"`
	Battery       int     `json:"battery"`
	Mode          string  `json:"mode"`
	State         string  `json:"state"`

	// 타겟 정보
	TargetEnemyID   string `json:"target_enemy_id"`
	TargetEnemyHP   int    `json:"target_enemy_hp"`
	TargetEnemyName string `json:"target_enemy_name"`

	// 명령 정보
	CommandType string  `json:"command_type"`
	TargetX     float64 `json:"target_x"`
	TargetY     float64 `json:"target_y"`

	// LLM 설명 (나중에 생성)
	AIExplanation string `json:"ai_explanation"`

	// 메타데이터
	DataJSON string `json:"data_json"` // 원본 메시지 JSON
	UserID   string `json:"user_id"`   // 웹 사용자 ID (옵션)
}

// LogSummary - 최근 로그 요약 (LLM용)
type LogSummary struct {
	RecentActions []AGVLog `json:"recent_actions"`
	Patterns      string   `json:"patterns"` // LLM이 분석한 패턴
}
