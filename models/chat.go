package models

import "time"

// ChatMessage - 채팅 메시지 구조 (추가 기능용)
type ChatMessage struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "user" or "ai"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id,omitempty"`
}

// ChatRequest - 사용자 채팅 요청 (추가 기능용)
type ChatRequest struct {
	Message   string `json:"message"`
	Context   string `json:"context,omitempty"` // AGV 상태 등 컨텍스트
	Timestamp int64  `json:"timestamp"`
}

// ChatResponse - AI 응답 (추가 기능용)
type ChatResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model"`
}

// AGVEventLog - AGV 이벤트 로그 (LLM 설명용)
type AGVEventLog struct {
	Timestamp   time.Time    `json:"timestamp"`
	EventType   string       `json:"event_type"` // "target_change", "path_start", "attack", "avoid_obstacle"
	Position    PositionData `json:"position"`
	Enemies     []Enemy      `json:"enemies,omitempty"` // ✅ Enemy로 수정
	Decision    string       `json:"decision"`          // 의사결정 근거
	Explanation string       `json:"explanation"`       // LLM 생성 설명
}

// LLMContext - LLM에 전달할 컨텍스트
type LLMContext struct {
	CurrentPosition PositionData  `json:"current_position"`
	TargetEnemy     *Enemy        `json:"target_enemy,omitempty"` // ✅ Enemy로 수정
	Enemies         []Enemy       `json:"enemies"`                // ✅ Enemy로 수정
	CurrentMode     string        `json:"current_mode"`           // "auto" or "manual"
	BatteryLevel    float64       `json:"battery_level"`
	Speed           float64       `json:"speed"`
	RecentEvents    []AGVEventLog `json:"recent_events"`
}
