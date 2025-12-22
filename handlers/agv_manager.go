package handlers

import (
	"fmt"
	"log"
	"sync"
	"time"

	"sion-backend/models"
)

// AGVManager - AGV(자율주행 로봇) 상태 관리
type AGVManager struct {
	mu       sync.RWMutex
	agvs     map[string]*AGVInfo  // agv_id -> AGVInfo
	lastPing map[string]time.Time // agv_id -> 마지막 Ping 시간
}

// AGVInfo - AGV의 정보
type AGVInfo struct {
	ID              string              `json:"id"`               // AGV ID
	RegisteredAt    time.Time           `json:"registered_at"`    // 등록 시간
	LastUpdate      time.Time           `json:"last_update"`      // 마지막 업데이트 시간
	Position        models.PositionData `json:"position"`         // 현재 위치
	Mode            models.AGVMode      `json:"mode"`            // auto/manual
	State           models.AGVState     `json:"state"`           // idle/moving/charging
	Battery         float64             `json:"battery"`         // 0-100%
	Speed           float64             `json:"speed"`           // m/s
	DetectedEnemies []models.Enemy      `json:"detected_enemies"` // 탐지된 적
}

// NewAGVManager - AGV Manager 생성
func NewAGVManager() *AGVManager {
	return &AGVManager{
		agvs:     make(map[string]*AGVInfo),
		lastPing: make(map[string]time.Time),
	}
}

// RegisterAGV - AGV 등록
//
// 새로운 AGV를 등록하거나 기존 AGV를 업데이트한다.
// agv_id가 이미 존재하면 기존 정보를 업데이트한다.
func (m *AGVManager) RegisterAGV(agvID string) (*AGVInfo, error) {
	if agvID == "" {
		return nil, fmt.Errorf("AGV ID가 비어있습니다")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	if info, exists := m.agvs[agvID]; exists {
		// 기존 AGV 업데이트
		info.LastUpdate = now
		m.lastPing[agvID] = now
		log.Printf("[Manager] AGV re-registered: %s\n", agvID)
		return info, nil
	}

	// 새 AGV 등록
	info := &AGVInfo{
		ID:           agvID,
		RegisteredAt: now,
		LastUpdate:   now,
		Position: models.PositionData{
			X:         0,
			Y:         0,
			Angle:     0,
			Timestamp: float64(now.UnixMilli()) / 1000.0, // Unix timestamp (seconds with ms)
		},
		Mode:    models.ModeAuto,
		State:   models.StateIdle,
		Battery: 100.0,
		Speed:   0.0,
	}

	m.agvs[agvID] = info
	m.lastPing[agvID] = now
	log.Printf("[Manager] AGV registered: %s\n", agvID)
	return info, nil
}

// UpdateStatus - AGV 상태 업데이트
//
// AGV의 현재 상태를 업데이트한다.
// 위치, 모드, 상태, 배터리, 속도, 탐지된 적 등을 업데이트한다.
func (m *AGVManager) UpdateStatus(
	agvID string,
	position models.PositionData,
	mode models.AGVMode,
	state models.AGVState,
	battery float64,
	speed float64,
	enemies []models.Enemy,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.agvs[agvID]
	if !exists {
		return fmt.Errorf("AGV not found: %s", agvID)
	}

	now := time.Now()

	// 위치 업데이트
	position.Timestamp = float64(now.UnixMilli()) / 1000.0 // Unix timestamp (seconds with ms)
	info.Position = position

	// 상태 업데이트
	if mode != "" {
		info.Mode = mode
	}
	if state != "" {
		info.State = state
	}

	// 배터리, 속도 업데이트
	info.Battery = battery
	info.Speed = speed
	info.DetectedEnemies = enemies
	info.LastUpdate = now
	m.lastPing[agvID] = now

	log.Printf("[Manager] AGV updated: %s (pos: %.2f, %.2f, bat: %.1f%%)\n",
		agvID, position.X, position.Y, battery)

	return nil
}

// GetStatus - AGV 상태 조회
//
// 특정 AGV의 현재 상태를 조회한다.
func (m *AGVManager) GetStatus(agvID string) (*AGVInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.agvs[agvID]
	if !exists {
		return nil, fmt.Errorf("AGV not found: %s", agvID)
	}

	return info, nil
}

// GetAllStatuses - 모든 AGV 상태 조회
//
// 현재 등록된 모든 AGV의 상태를 조회한다.
func (m *AGVManager) GetAllStatuses() []*AGVInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AGVInfo, 0, len(m.agvs))
	for _, info := range m.agvs {
		result = append(result, info)
	}

	return result
}

// RemoveAGV - AGV 등록 해제
//
// 특정 AGV를 등록 해제한다.
func (m *AGVManager) RemoveAGV(agvID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agvs[agvID]; !exists {
		return fmt.Errorf("AGV not found: %s", agvID)
	}

	delete(m.agvs, agvID)
	delete(m.lastPing, agvID)
	log.Printf("[Manager] AGV removed: %s\n", agvID)

	return nil
}

// GetAGVCount - 현재 등록된 AGV 수
func (m *AGVManager) GetAGVCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.agvs)
}

// IsAGVAlive - AGV가 살아있는지 확인
//
// 마지막 Ping으로부터 타임아웃 시간 내에 있는지 확인한다.
// 기본 타임아웃: 10초
func (m *AGVManager) IsAGVAlive(agvID string, timeout time.Duration) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lastPing, exists := m.lastPing[agvID]
	if !exists {
		return false
	}

	return time.Since(lastPing) < timeout
}

// CleanupOfflineAGVs - 오프라인 AGV 정리
//
// 주어진 타임아웃 시간 동안 신호를 보내지 않은 AGV를 제거한다.
func (m *AGVManager) CleanupOfflineAGVs(timeout time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()

	for agvID, lastPing := range m.lastPing {
		if now.Sub(lastPing) > timeout {
			delete(m.agvs, agvID)
			delete(m.lastPing, agvID)
			log.Printf("[Manager] AGV cleanup: %s (offline)\n", agvID)
			count++
		}
	}

	return count
}

// GetConnectedAGVs - 연결된 AGV 목록
func (m *AGVManager) GetConnectedAGVs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0, len(m.agvs))
	for agvID := range m.agvs {
		result = append(result, agvID)
	}

	return result
}

// SendCommandToAGV - AGV에게 명령 전송
//
// 특정 AGV에게 명령을 전송한다.
func (m *AGVManager) SendCommandToAGV(agvID string, cmd interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.agvs[agvID]; !exists {
		return fmt.Errorf("AGV not found: %s", agvID)
	}

	// TODO: 실제 명령 전송 로직 구현
	log.Printf("[Manager] Command sent to AGV %s: %v\n", agvID, cmd)

	return nil
}

// BroadcastCommandToAllAGVs - 모든 AGV에게 명령 브로드캐스트
func (m *AGVManager) BroadcastCommandToAllAGVs(cmd interface{}) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for agvID := range m.agvs {
		log.Printf("[Manager] Broadcasting to AGV %s: %v\n", agvID, cmd)
		count++
	}

	return count
}

// GetStatistics - AGV 통계
func (m *AGVManager) GetStatistics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalCount := len(m.agvs)
	autoCount := 0
	idleCount := 0
	totalBattery := 0.0

	for _, info := range m.agvs {
		if info.Mode == models.ModeAuto {
			autoCount++
		}
		if info.State == models.StateIdle {
			idleCount++
		}
		totalBattery += info.Battery
	}

	avgBattery := 0.0
	if totalCount > 0 {
		avgBattery = totalBattery / float64(totalCount)
	}

	return map[string]interface{}{
		"total_agvs":    totalCount,
		"auto_mode":     autoCount,
		"idle_state":    idleCount,
		"avg_battery":   avgBattery,
		"total_battery": totalBattery,
	}
}
