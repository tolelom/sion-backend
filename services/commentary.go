package services

import (
	"fmt"
	"log"
	"math"
	"sion-backend/models"
	"sync"
	"time"
)

// CommentaryService - AGV 행동 자동 중계 서비스
type CommentaryService struct {
	llmService    *LLMService
	broadcastFunc func(models.WebSocketMessage)

	// 상태 추적
	lastPosition   models.PositionData
	lastState      models.AGVState
	lastTargetID   string
	lastBattery    int
	lastCommentary time.Time

	// 설정
	cooldown time.Duration // 해설 간격 (너무 자주 해설하지 않도록)
	enabled  bool
	mu       sync.RWMutex

	// 이벤트 큐
	eventQueue chan CommentaryEvent
	stopChan   chan bool
}

// CommentaryEvent - 해설 이벤트
type CommentaryEvent struct {
	Type      string                 // 이벤트 타입
	Priority  int                    // 우선순위 (높을수록 먼저 처리)
	Data      map[string]interface{} // 이벤트 데이터
	Timestamp time.Time
}

// 이벤트 타입 상수
const (
	EventTargetFound    = "target_found"    // 적 발견
	EventTargetChanged  = "target_changed"  // 타겟 변경
	EventTargetDefeated = "target_defeated" // 적 처치
	EventChargingStart  = "charging_start"  // 돌진 시작
	EventChargingEnd    = "charging_end"    // 돌진 종료
	EventLowBattery     = "low_battery"     // 배터리 부족
	EventModeChanged    = "mode_changed"    // 모드 변경
	EventPathStart      = "path_start"      // 경로 이동 시작
	EventPathComplete   = "path_complete"   // 경로 도착
	EventObstacleNear   = "obstacle_near"   // 장애물 접근
	EventIdle           = "idle_status"     // 대기 상태 진입
	EventPeriodicUpdate = "periodic_update" // 주기적 상황 요약
)

// 이벤트 우선순위
var eventPriority = map[string]int{
	EventTargetDefeated: 100, // 최고 우선순위
	EventChargingStart:  90,
	EventTargetFound:    80,
	EventTargetChanged:  70,
	EventLowBattery:     60,
	EventModeChanged:    50,
	EventPathStart:      40,
	EventPathComplete:   30,
	EventObstacleNear:   20,
	EventIdle:           10,
	EventPeriodicUpdate: 5, // 최저 우선순위
}

// NewCommentaryService - 자동 중계 서비스 생성
func NewCommentaryService(llmService *LLMService, broadcastFunc func(models.WebSocketMessage)) *CommentaryService {
	return &CommentaryService{
		llmService:     llmService,
		broadcastFunc:  broadcastFunc,
		cooldown:       5 * time.Second, // 기본 5초 쿨다운
		enabled:        true,
		eventQueue:     make(chan CommentaryEvent, 50),
		stopChan:       make(chan bool),
		lastCommentary: time.Now().Add(-10 * time.Second), // 시작 시 바로 해설 가능
	}
}

// Start - 자동 중계 서비스 시작
func (cs *CommentaryService) Start() {
	log.Println("🎙️ 자동 중계 서비스 시작")
	go cs.processEvents()
	go cs.periodicCommentary()
}

// Stop - 자동 중계 서비스 중지
func (cs *CommentaryService) Stop() {
	cs.stopChan <- true
	log.Println("🎙️ 자동 중계 서비스 중지")
}

// SetEnabled - 자동 중계 활성화/비활성화
func (cs *CommentaryService) SetEnabled(enabled bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.enabled = enabled
	if enabled {
		log.Println("🎙️ 자동 중계 활성화")
	} else {
		log.Println("🎙️ 자동 중계 비활성화")
	}
}

// SetCooldown - 해설 쿨다운 설정
func (cs *CommentaryService) SetCooldown(duration time.Duration) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.cooldown = duration
}

// processEvents - 이벤트 큐 처리
func (cs *CommentaryService) processEvents() {
	for {
		select {
		case event := <-cs.eventQueue:
			cs.handleEvent(event)
		case <-cs.stopChan:
			return
		}
	}
}

// periodicCommentary - 주기적 상황 요약 (30초마다)
func (cs *CommentaryService) periodicCommentary() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cs.mu.RLock()
			enabled := cs.enabled
			cs.mu.RUnlock()

			if enabled {
				cs.QueueEvent(EventPeriodicUpdate, map[string]interface{}{
					"type": "periodic",
				})
			}
		case <-cs.stopChan:
			return
		}
	}
}

// QueueEvent - 이벤트 큐에 추가
func (cs *CommentaryService) QueueEvent(eventType string, data map[string]interface{}) {
	cs.mu.RLock()
	enabled := cs.enabled
	cs.mu.RUnlock()

	if !enabled {
		return
	}

	priority := eventPriority[eventType]
	if priority == 0 {
		priority = 10
	}

	event := CommentaryEvent{
		Type:      eventType,
		Priority:  priority,
		Data:      data,
		Timestamp: time.Now(),
	}

	// 비차단 방식으로 큐에 추가
	select {
	case cs.eventQueue <- event:
		log.Printf("🎙️ 이벤트 큐 추가: %s (우선순위: %d)", eventType, priority)
	default:
		log.Printf("⚠️ 이벤트 큐 가득 참, 이벤트 무시: %s", eventType)
	}
}

// handleEvent - 이벤트 처리 및 해설 생성
func (cs *CommentaryService) handleEvent(event CommentaryEvent) {
	cs.mu.Lock()
	// 쿨다운 체크
	if time.Since(cs.lastCommentary) < cs.cooldown {
		cs.mu.Unlock()
		log.Printf("🎙️ 쿨다운 중, 이벤트 스킵: %s", event.Type)
		return
	}
	cs.lastCommentary = time.Now()
	cs.mu.Unlock()

	// LLM 서비스 확인
	if cs.llmService == nil {
		log.Println("⚠️ LLM 서비스가 없어 해설 생성 불가")
		return
	}

	// 프롬프트 생성
	prompt := cs.buildPrompt(event)
	if prompt == "" {
		return
	}

	// LLM 호출 (비동기)
	go func() {
		commentary, err := cs.generateCommentary(event.Type, prompt)
		if err != nil {
			log.Printf("❌ 해설 생성 실패: %v", err)
			return
		}

		// WebSocket으로 브로드캐스트
		cs.broadcastCommentary(event.Type, commentary)

		// DB에 로그 저장
		LogAIExplanation("sion-001", event.Type, commentary)
	}()
}

// buildPrompt - 이벤트별 프롬프트 생성
func (cs *CommentaryService) buildPrompt(event CommentaryEvent) string {
	data := event.Data

	switch event.Type {
	case EventTargetFound:
		enemyName := getStringFromMap(data, "enemy_name", "적")
		enemyHP := getIntFromMap(data, "enemy_hp", 100)
		distance := getFloatFromMap(data, "distance", 0)
		return fmt.Sprintf(`[적 발견! 🎯]
사이온이 %s을(를) 발견했습니다!
- 적 체력: %d%%
- 거리: %.1fm
이 상황을 e스포츠 캐스터처럼 흥분되게 해설해주세요. 2문장으로.`, enemyName, enemyHP, distance)

	case EventTargetChanged:
		oldTarget := getStringFromMap(data, "old_target", "이전 타겟")
		newTarget := getStringFromMap(data, "new_target", "새 타겟")
		reason := getStringFromMap(data, "reason", "전략적 판단")
		return fmt.Sprintf(`[타겟 변경! 🔄]
사이온이 타겟을 %s에서 %s(으)로 변경했습니다!
- 변경 이유: %s
왜 이런 결정을 했는지 e스포츠 캐스터처럼 분석해주세요. 2문장으로.`, oldTarget, newTarget, reason)

	case EventTargetDefeated:
		enemyName := getStringFromMap(data, "enemy_name", "적")
		return fmt.Sprintf(`[적 처치! ⚔️]
사이온이 %s을(를) 처치했습니다!
승리의 순간을 e스포츠 캐스터처럼 열정적으로 해설해주세요. 2문장으로.`, enemyName)

	case EventChargingStart:
		targetName := getStringFromMap(data, "target_name", "타겟")
		speed := getFloatFromMap(data, "speed", 2.5)
		return fmt.Sprintf(`[궁극기 발동! 🚀]
사이온이 "멈출 수 없는 맹공"을 시전합니다!
- 타겟: %s
- 돌진 속도: %.1f m/s
이 결정적 순간을 e스포츠 캐스터처럼 흥분되게 해설해주세요. 2문장으로.`, targetName, speed)

	case EventLowBattery:
		battery := getIntFromMap(data, "battery", 20)
		return fmt.Sprintf(`[배터리 경고! 🔋]
사이온의 배터리가 %d%%로 떨어졌습니다!
위기 상황을 e스포츠 캐스터처럼 긴장감 있게 해설해주세요. 2문장으로.`, battery)

	case EventModeChanged:
		newMode := getStringFromMap(data, "mode", "auto")
		modeKR := "자동 모드"
		if newMode == "manual" {
			modeKR = "수동 모드"
		}
		return fmt.Sprintf(`[모드 변경! 🎮]
사이온이 %s로 전환했습니다!
이 전략적 변경을 e스포츠 캐스터처럼 해설해주세요. 2문장으로.`, modeKR)

	case EventPathStart:
		targetX := getFloatFromMap(data, "target_x", 0)
		targetY := getFloatFromMap(data, "target_y", 0)
		return fmt.Sprintf(`[이동 시작! 🏃]
사이온이 (%.1f, %.1f) 지점으로 이동을 시작합니다!
이동 상황을 간략히 해설해주세요. 1문장으로.`, targetX, targetY)

	case EventPeriodicUpdate:
		return `[상황 요약 📊]
현재 사이온의 전투 상황을 간략히 요약해주세요.
e스포츠 캐스터처럼 현재 전황을 분석해주세요. 2문장으로.`

	default:
		return fmt.Sprintf(`[이벤트: %s]
현재 상황을 e스포츠 캐스터처럼 해설해주세요. 1-2문장으로.`, event.Type)
	}
}

// generateCommentary - LLM으로 해설 생성
func (cs *CommentaryService) generateCommentary(eventType, prompt string) (string, error) {
	systemPrompt := `당신은 AGV 로봇 "사이온"의 실시간 e스포츠 해설자입니다.

🎙️ 해설 스타일:
- 열정적이고 흥분된 톤
- 짧고 임팩트 있는 문장
- 리그오브레전드 사이온 캐릭터의 특성 반영 (강인함, 불굴의 의지)
- 한국어 e스포츠 중계 스타일

⚠️ 주의사항:
- 반드시 지정된 문장 수를 지켜주세요
- 기술적인 용어보다 재미있는 표현 사용
- 이모지를 적절히 사용`

	return cs.llmService.callOllama(systemPrompt, prompt)
}

// broadcastCommentary - 해설 브로드캐스트
func (cs *CommentaryService) broadcastCommentary(eventType, commentary string) {
	if cs.broadcastFunc == nil {
		return
	}

	msg := models.WebSocketMessage{
		Type: models.MessageTypeLLMExplanation,
		Data: models.LLMExplanation{
			Text:      commentary,
			Action:    eventType,
			Reason:    "auto_commentary",
			Timestamp: time.Now().UnixMilli(),
		},
		Timestamp: time.Now().UnixMilli(),
	}

	cs.broadcastFunc(msg)
	log.Printf("🎙️ 해설 전송: [%s] %s", eventType, truncateString(commentary, 50))
}

// ============================================
// AGV 상태 변화 감지 메서드들
// ============================================

// OnAGVStatusUpdate - AGV 상태 업데이트 시 호출
func (cs *CommentaryService) OnAGVStatusUpdate(status *models.AGVStatus) {
	if status == nil {
		return
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 1. 상태 변화 감지 (idle → moving 등)
	if cs.lastState != "" && cs.lastState != status.State {
		if status.State == models.StateCharging {
			cs.mu.Unlock()
			cs.QueueEvent(EventChargingStart, map[string]interface{}{
				"target_name": getTargetName(status.TargetEnemy),
				"speed":       status.Speed,
			})
			cs.mu.Lock()
		}
	}
	cs.lastState = status.State

	// 2. 타겟 변경 감지
	currentTargetID := ""
	if status.TargetEnemy != nil {
		currentTargetID = status.TargetEnemy.ID
	}
	if cs.lastTargetID != "" && cs.lastTargetID != currentTargetID && currentTargetID != "" {
		cs.mu.Unlock()
		cs.QueueEvent(EventTargetChanged, map[string]interface{}{
			"old_target": cs.lastTargetID,
			"new_target": getTargetName(status.TargetEnemy),
			"reason":     "더 낮은 체력의 적 발견",
		})
		cs.mu.Lock()
	}
	cs.lastTargetID = currentTargetID

	// 3. 배터리 부족 감지
	if cs.lastBattery > 20 && status.Battery <= 20 {
		cs.mu.Unlock()
		cs.QueueEvent(EventLowBattery, map[string]interface{}{
			"battery": status.Battery,
		})
		cs.mu.Lock()
	}
	cs.lastBattery = status.Battery

	// 4. 위치 업데이트
	cs.lastPosition = status.Position
}

// OnTargetFound - 적 발견 시 호출
func (cs *CommentaryService) OnTargetFound(enemy *models.Enemy, distance float64) {
	if enemy == nil {
		return
	}

	cs.QueueEvent(EventTargetFound, map[string]interface{}{
		"enemy_name": enemy.Name,
		"enemy_hp":   enemy.HP,
		"distance":   distance,
	})
}

// OnTargetDefeated - 적 처치 시 호출
func (cs *CommentaryService) OnTargetDefeated(enemy *models.Enemy) {
	if enemy == nil {
		return
	}

	cs.QueueEvent(EventTargetDefeated, map[string]interface{}{
		"enemy_name": enemy.Name,
	})
}

// OnModeChanged - 모드 변경 시 호출
func (cs *CommentaryService) OnModeChanged(newMode string) {
	cs.QueueEvent(EventModeChanged, map[string]interface{}{
		"mode": newMode,
	})
}

// ============================================
// 헬퍼 함수들
// ============================================

func getStringFromMap(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func getIntFromMap(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return defaultVal
}

func getFloatFromMap(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return defaultVal
}

func getTargetName(enemy *models.Enemy) string {
	if enemy == nil {
		return "알 수 없는 적"
	}
	return enemy.Name
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func calculateDistanceBetween(pos1, pos2 models.PositionData) float64 {
	dx := pos1.X - pos2.X
	dy := pos1.Y - pos2.Y
	return math.Sqrt(dx*dx + dy*dy)
}
