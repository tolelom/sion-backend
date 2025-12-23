package services

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sion-backend/models"
	"sync"
	"time"
)

// AGVSimulator - AGV 시뮬레이터
type AGVSimulator struct {
	IsRunning         bool
	broadcastFunc     func(models.WebSocketMessage)
	commentaryService *CommentaryService // 🆕 자동 중계 서비스

	// 시뮬레이션 상태
	position models.PositionData
	target   *models.PositionData
	state    models.AGVState
	mode     models.AGVMode
	battery  int
	speed    float64

	// 적 정보
	enemies     []*models.Enemy
	targetEnemy *models.Enemy

	// 제어
	stopChan chan bool
	mu       sync.RWMutex
}

// NewAGVSimulator - 시뮬레이터 생성
func NewAGVSimulator(broadcastFunc func(models.WebSocketMessage)) *AGVSimulator {
	return &AGVSimulator{
		broadcastFunc: broadcastFunc,
		position: models.PositionData{
			X:     5.0,
			Y:     5.0,
			Angle: 0,
		},
		state:    models.StateIdle,
		mode:     models.ModeAuto,
		battery:  100,
		speed:    0,
		stopChan: make(chan bool),
		enemies:  generateInitialEnemies(),
	}
}

// SetCommentaryService - 자동 중계 서비스 설정
func (s *AGVSimulator) SetCommentaryService(cs *CommentaryService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commentaryService = cs
	log.Println("🎙️ 시뮬레이터에 자동 중계 서비스 연결됨")
}

// Start - 시뮬레이션 시작
func (s *AGVSimulator) Start() {
	s.mu.Lock()
	if s.IsRunning {
		s.mu.Unlock()
		return
	}
	s.IsRunning = true
	s.mu.Unlock()

	log.Println("🚀 AGV 시뮬레이터 시작")

	// 🆕 시작 해설
	s.triggerCommentary("charging_start", map[string]interface{}{
		"target_name": "전장",
		"speed":       2.5,
	})

	go s.runSimulation()
}

// Stop - 시뮬레이션 중지
func (s *AGVSimulator) Stop() {
	s.mu.Lock()
	if !s.IsRunning {
		s.mu.Unlock()
		return
	}
	s.IsRunning = false
	s.mu.Unlock()

	s.stopChan <- true
	log.Println("🛑 AGV 시뮬레이터 중지")
}

// runSimulation - 시뮬레이션 메인 루프
func (s *AGVSimulator) runSimulation() {
	ticker := time.NewTicker(100 * time.Millisecond) // 10Hz 업데이트
	defer ticker.Stop()

	scanTicker := time.NewTicker(2 * time.Second) // 2초마다 적 스캔
	defer scanTicker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.update()
		case <-scanTicker.C:
			s.scanForEnemies()
		}
	}
}

// update - 시뮬레이션 업데이트
func (s *AGVSimulator) update() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 배터리 감소
	if s.state != models.StateIdle {
		s.battery -= rand.Intn(2) // 0 또는 1 감소
		if s.battery < 0 {
			s.battery = 0
		}

		// 🆕 배터리 20% 이하 경고
		if s.battery == 20 {
			go s.triggerCommentary("low_battery", map[string]interface{}{
				"battery": s.battery,
			})
		}
	}

	// 타겟이 있으면 추적
	if s.targetEnemy != nil {
		s.chaseTarget()
	} else if s.target != nil {
		s.moveToTarget()
	} else {
		s.state = models.StateIdle
		s.speed = 0
	}

	// 위치 브로드캐스트
	s.broadcastPosition()
	s.broadcastStatus()
}

// chaseTarget - 타겟 추적
func (s *AGVSimulator) chaseTarget() {
	if s.targetEnemy == nil {
		return
	}

	// 타겟 방향 계산
	dx := s.targetEnemy.Position.X - s.position.X
	dy := s.targetEnemy.Position.Y - s.position.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// 타겟 도달 시 처치
	if distance < 0.5 {
		enemyName := s.targetEnemy.Name
		s.targetEnemy.HP -= 25

		if s.targetEnemy.HP <= 0 {
			// 🆕 적 처치 해설
			go s.triggerCommentary("target_defeated", map[string]interface{}{
				"enemy_name": enemyName,
			})

			// 적 제거
			s.removeEnemy(s.targetEnemy.ID)
			s.targetEnemy = nil
			s.state = models.StateIdle
		}
		return
	}

	// 돌진 상태로 이동
	s.state = models.StateCharging
	s.speed = 2.5 // 궁극기 속도

	// 이동
	s.position.Angle = math.Atan2(dy, dx)
	moveSpeed := s.speed * 0.1 // 100ms 간격
	s.position.X += (dx / distance) * moveSpeed
	s.position.Y += (dy / distance) * moveSpeed
	s.position.Timestamp = float64(time.Now().UnixMilli()) / 1000.0
}

// moveToTarget - 일반 이동
func (s *AGVSimulator) moveToTarget() {
	if s.target == nil {
		return
	}

	dx := s.target.X - s.position.X
	dy := s.target.Y - s.position.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	if distance < 0.3 {
		// 🆕 목적지 도착 해설
		go s.triggerCommentary("path_complete", map[string]interface{}{
			"target_x": s.target.X,
			"target_y": s.target.Y,
		})

		s.target = nil
		s.state = models.StateIdle
		s.speed = 0
		return
	}

	s.state = models.StateMoving
	s.speed = 1.5

	moveSpeed := s.speed * 0.1
	s.position.Angle = math.Atan2(dy, dx)
	s.position.X += (dx / distance) * moveSpeed
	s.position.Y += (dy / distance) * moveSpeed
	s.position.Timestamp = float64(time.Now().UnixMilli()) / 1000.0
}

// scanForEnemies - 적 스캔
func (s *AGVSimulator) scanForEnemies() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mode != models.ModeAuto {
		return
	}

	var closestEnemy *models.Enemy
	closestDistance := math.MaxFloat64

	for _, enemy := range s.enemies {
		if enemy.HP <= 0 {
			continue
		}

		dx := enemy.Position.X - s.position.X
		dy := enemy.Position.Y - s.position.Y
		distance := math.Sqrt(dx*dx + dy*dy)

		// 감지 범위 내 (10m)
		if distance < 10.0 && distance < closestDistance {
			closestEnemy = enemy
			closestDistance = distance
		}
	}

	// 새로운 타겟 발견
	if closestEnemy != nil && (s.targetEnemy == nil || s.targetEnemy.ID != closestEnemy.ID) {
		oldTarget := s.targetEnemy
		s.targetEnemy = closestEnemy

		if oldTarget == nil {
			// 🆕 적 발견 해설
			go s.triggerCommentary("target_found", map[string]interface{}{
				"enemy_name": closestEnemy.Name,
				"enemy_hp":   closestEnemy.HP,
				"distance":   closestDistance,
			})
		} else {
			// 🆕 타겟 변경 해설
			go s.triggerCommentary("target_changed", map[string]interface{}{
				"old_target": oldTarget.Name,
				"new_target": closestEnemy.Name,
				"reason":     "더 가까운 적 발견",
			})
		}
	}
}

// SetTarget - 이동 목표 설정
func (s *AGVSimulator) SetTarget(x, y float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.target = &models.PositionData{X: x, Y: y}
	s.targetEnemy = nil // 수동 이동 시 적 추적 해제

	// 🆕 이동 시작 해설
	go s.triggerCommentary("path_start", map[string]interface{}{
		"target_x": x,
		"target_y": y,
	})

	log.Printf("📍 목표 설정: (%.1f, %.1f)", x, y)
}

// SetMode - 모드 변경
func (s *AGVSimulator) SetMode(mode models.AGVMode) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mode == mode {
		return
	}

	s.mode = mode

	// 🆕 모드 변경 해설
	go s.triggerCommentary("mode_changed", map[string]interface{}{
		"mode": string(mode),
	})

	log.Printf("🎮 모드 변경: %s", mode)
}

// broadcastPosition - 위치 브로드캐스트
func (s *AGVSimulator) broadcastPosition() {
	if s.broadcastFunc == nil {
		return
	}

	msg := models.WebSocketMessage{
		Type:      models.MessageTypePosition,
		Data:      s.position,
		Timestamp: time.Now().UnixMilli(),
	}
	s.broadcastFunc(msg)
}

// broadcastStatus - 상태 브로드캐스트
func (s *AGVSimulator) broadcastStatus() {
	if s.broadcastFunc == nil {
		return
	}

	var targetInfo map[string]interface{}
	if s.targetEnemy != nil {
		targetInfo = map[string]interface{}{
			"id":   s.targetEnemy.ID,
			"name": s.targetEnemy.Name,
			"hp":   s.targetEnemy.HP,
		}
	}

	msg := models.WebSocketMessage{
		Type: models.MessageTypeStatus,
		Data: map[string]interface{}{
			"battery":      s.battery,
			"speed":        s.speed,
			"mode":         s.mode,
			"state":        s.state,
			"target_enemy": targetInfo,
		},
		Timestamp: time.Now().UnixMilli(),
	}
	s.broadcastFunc(msg)
}

// triggerCommentary - 자동 중계 트리거
func (s *AGVSimulator) triggerCommentary(eventType string, data map[string]interface{}) {
	if s.commentaryService != nil {
		s.commentaryService.QueueEvent(eventType, data)
	}
}

// removeEnemy - 적 제거
func (s *AGVSimulator) removeEnemy(id string) {
	for i, enemy := range s.enemies {
		if enemy.ID == id {
			s.enemies = append(s.enemies[:i], s.enemies[i+1:]...)
			break
		}
	}
}

// GetStatus - 현재 상태 반환
func (s *AGVSimulator) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"running":  s.IsRunning,
		"position": s.position,
		"state":    s.state,
		"mode":     s.mode,
		"battery":  s.battery,
		"speed":    s.speed,
		"enemies":  len(s.enemies),
	}
}

// generateInitialEnemies - 초기 적 생성
func generateInitialEnemies() []*models.Enemy {
	names := []string{"아리", "야스오", "티모", "리신", "제드"}
	enemies := make([]*models.Enemy, len(names))

	for i, name := range names {
		enemies[i] = &models.Enemy{
			ID:   fmt.Sprintf("enemy-%d", i+1),
			Name: name,
			HP:   100,
			Position: models.PositionData{
				X: rand.Float64()*15 + 2,
				Y: rand.Float64()*15 + 2,
			},
		}
	}

	return enemies
}
