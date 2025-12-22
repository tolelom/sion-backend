package handlers

import (
	"encoding/json"
	"log"
	"sion-backend/models"
	"sion-backend/services"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

type Client struct {
	Conn       *websocket.Conn
	ClientType string // "agv" 또는 "web"
	AGVID      string // AGV 타입일 경우 AGV ID
}

// 클라이언트 관리자
type ClientManager struct {
	clients    map[*websocket.Conn]*Client
	broadcast  chan models.WebSocketMessage
	register   chan *Client
	unregister chan *websocket.Conn
	mutex      sync.RWMutex
}

// 전역 클라이언트 관리자
var Manager = &ClientManager{
	clients:    make(map[*websocket.Conn]*Client),
	broadcast:  make(chan models.WebSocketMessage, 256),
	register:   make(chan *Client),
	unregister: make(chan *websocket.Conn),
}

// 전역 AGV Manager (main.go에서 초기화)
var AGVMgr *AGVManager

// 클라이언트 관리 시작
func (manager *ClientManager) Start() {
	for {
		select {
		case client := <-manager.register:
			manager.mutex.Lock()
			manager.clients[client.Conn] = client
			manager.mutex.Unlock()
			log.Printf("[Manager] 클라이언트 등록: %s (%s)", client.ClientType, client.Conn.RemoteAddr())

		case conn := <-manager.unregister:
			manager.mutex.Lock()
			if client, ok := manager.clients[conn]; ok {
				delete(manager.clients, conn)
				_ = conn.Close()
				// AGV 연결 해제 시 Manager에서도 제거
				if client.ClientType == "agv" && client.AGVID != "" && AGVMgr != nil {
					_ = AGVMgr.RemoveAGV(client.AGVID)
				}
				log.Printf("[Manager] 클라이언트 해제: %s (%s)", client.ClientType, conn.RemoteAddr())
			}
			manager.mutex.Unlock()

		case message := <-manager.broadcast:
			manager.handleBroadcast(message)
		}
	}
}

func (manager *ClientManager) handleBroadcast(message models.WebSocketMessage) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	for conn, client := range manager.clients {
		// 메시지 타입에 따라 전송 대상 결정
		shouldSend := false

		switch message.Type {
		case models.MessageTypePosition,
			models.MessageTypeStatus,
			models.MessageTypeLog,
			models.MessageTypeTargetFound,
			models.MessageTypePathUpdate,
			models.MessageTypeChatResponse,
			models.MessageTypeAGVEvent:
			// AGV에서 Web으로 전송
			if client.ClientType == "web" {
				shouldSend = true
			}
		case models.MessageTypeCommand,
			models.MessageTypeModeChange,
			models.MessageTypeEmergencyStop:
			// Web에서 AGV로 전송
			if client.ClientType == "agv" {
				shouldSend = true
			}
		case models.MessageTypeLLMExplanation,
			models.MessageTypeTTS,
			models.MessageTypeMapUpdate,
			models.MessageTypeSystemInfo,
			"agv_status_update": // ★ Frontend가 대기하는 타입
			// 모든 Web 클라이언트에게 전송
			if client.ClientType == "web" {
				shouldSend = true
			}
		}

		if shouldSend {
			err := conn.WriteJSON(message)
			if err != nil {
				log.Printf("[Manager] 전송 실패 (%s): %v", client.ClientType, err)
				manager.unregister <- conn
			}
		}
	}
}

// 외부에서 호출할 수 있는 브로드캐스트 메서드
func (manager *ClientManager) BroadcastMessage(msg models.WebSocketMessage) {
	manager.broadcast <- msg
}

func (manager *ClientManager) GetClientCount() map[string]int {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	count := map[string]int{
		"agv": 0,
		"web": 0,
	}

	for _, client := range manager.clients {
		count[client.ClientType]++
	}

	return count
}

// AGV WebSocket Handler
func HandleAGVWebSocket(c *websocket.Conn) {
	client := &Client{
		Conn:       c,
		ClientType: "agv",
	}

	Manager.register <- client

	defer func() {
		Manager.unregister <- c
	}()

	var agvID string
	var isRegistered bool

	for {
		var msg models.WebSocketMessage
		err := c.ReadJSON(&msg)
		if err != nil {
			log.Printf("[AGV] 메시지 읽기 오류: %v", err)
			break
		}

		// 타임스탬프 추가
		if msg.Timestamp == 0 {
			msg.Timestamp = time.Now().UnixMilli()
		}

		log.Printf("[AGV] 메시지 타입: %s, 데이터: %+v", msg.Type, msg.Data)

		// 처음 메시지: registration 또는 status
		switch msg.Type {
		case "registration":
			// AGV 등록
			log.Printf("[AGV] 🔍 Registration 메시지 처리 시작")
			
			data, err := json.Marshal(msg.Data)
			if err != nil {
				log.Printf("[AGV] JSON 마샬링 실패: %v", err)
				continue
			}

			log.Printf("[AGV] Raw registration data: %s", string(data)) // 디버깅용

			var reg models.AGVRegistration
			err = json.Unmarshal(data, &reg)
			if err != nil {
				log.Printf("[AGV] 등록 메시지 파싱 실패: %v", err)
				log.Printf("[AGV] Expected: AgentID, optional Mode, Position, Timestamp")
				continue
			}

			// ★ Mode가 없으면 기본값 설정
			if reg.Mode == "" {
				reg.Mode = models.ModeAuto
				log.Printf("[AGV] Mode가 없음, 기본값 설정: %s", models.ModeAuto)
			}

			log.Printf("[AGV] Parsed - AgentID: %s, Mode: %s, Position: (%.2f, %.2f)",
				reg.AgentID, reg.Mode, reg.Position.X, reg.Position.Y)

			if AGVMgr != nil {
				_, err := AGVMgr.RegisterAGV(reg.AgentID)
				if err != nil {
					log.Printf("[AGV] 등록 실패: %v", err)
					continue
				}

				// ★ 중요: 이 부분이 실행되어야 isRegistered가 true가 됨
				agvID = reg.AgentID
				client.AGVID = agvID
				isRegistered = true

				log.Printf("[AGV] ✅ 등록 완료: %s (isRegistered=%v, Position: %.2f, %.2f)",
					reg.AgentID, isRegistered, reg.Position.X, reg.Position.Y)

				// 웹 클라이언트에 알림
				notifyMsg := models.WebSocketMessage{
					Type: models.MessageTypeSystemInfo,
					Data: map[string]interface{}{
						"event":  "agv_registered",
						"agv_id": agvID,
					},
					Timestamp: time.Now().UnixMilli(),
				}
				Manager.BroadcastMessage(notifyMsg)
			}

		case models.MessageTypeStatus:
			// AGV 상태 업데이트
			if !isRegistered || agvID == "" {
				log.Printf("[AGV] ⚠️  상태 업데이트 전 등록 필요 (isRegistered=%v, agvID=%s)", isRegistered, agvID)
				continue
			}

			log.Printf("[AGV] Status 메시지 처리: isRegistered=%v, agvID=%s", isRegistered, agvID)

			// Status 메시지 파싱
			data, err := json.Marshal(msg.Data)
			if err != nil {
				log.Printf("[AGV] JSON 마샬링 실패: %v", err)
				continue
			}

			var statusData map[string]interface{}
			err = json.Unmarshal(data, &statusData)
			if err != nil {
				log.Printf("[AGV] 상태 메시지 파싱 실패: %v", err)
				continue
			}

			// 위치 추출
			var pos models.PositionData
			if posData, ok := statusData["position"]; ok {
				posBytes, _ := json.Marshal(posData)
				json.Unmarshal(posBytes, &pos)
			}

			// 상태 업데이트
			var mode models.AGVMode = models.ModeAuto
			var state models.AGVState = models.StateIdle
			var battery float64 = 100.0
			var speed float64 = 0.0

			if m, ok := statusData["mode"]; ok {
				if str, ok := m.(string); ok {
					mode = models.AGVMode(str)
				}
			}
			if s, ok := statusData["state"]; ok {
				if str, ok := s.(string); ok {
					state = models.AGVState(str)
				}
			}
			if b, ok := statusData["battery"]; ok {
				if bf, ok := b.(float64); ok {
					battery = bf
				}
			}
			if spd, ok := statusData["speed"]; ok {
				if sf, ok := spd.(float64); ok {
					speed = sf
				}
			}

			if AGVMgr != nil {
				err := AGVMgr.UpdateStatus(
					agvID,
					pos,
					mode,
					state,
					battery,
					speed,
					[]models.Enemy{},
				)
				if err != nil {
					log.Printf("[AGV] 상태 업데이트 실패: %v", err)
					continue // ★ 오류 시 진행 중단
				}

				// ★ 중요: 모든 웹 클라이언트에게 명시적으로 AGV 상태 브로드캐스트
				// Frontend에서 "agv_status_update" 타입을 대기 중
				statuses := AGVMgr.GetAllStatuses()
				if len(statuses) > 0 {
					statusMsg := models.WebSocketMessage{
						Type: "agv_status_update", // ★ Frontend가 인식하는 타입
						Data: map[string]interface{}{
							"agvs": statuses,
						},
						Timestamp: time.Now().UnixMilli(),
					}
					Manager.BroadcastMessage(statusMsg)
					log.Printf("[AGV] 웹에 브로드캐스트: %d개 AGV 상태", len(statuses))
				}
			}

			// 로깅만 수행 (원본 메시지는 브로드캐스트하지 않음)
			go services.LogAGVEvent(msg, agvID, "agv")

			// ★ 원본 "status" 메시지는 브로드캐스트하지 않음

		default:
			log.Printf("[AGV] 알 수 없는 메시지 타입: %s", msg.Type)
			// 다른 메시지도 브로드캐스트
			go services.LogAGVEvent(msg, agvID, "agv")
			Manager.BroadcastMessage(msg)
		}
	}
}

// Web 클라이언트 WebSocket Handler (채팅 + LLM 연동)
func HandleWebClientWebSocket(c *websocket.Conn) {
	client := &Client{
		Conn:       c,
		ClientType: "web",
	}

	Manager.register <- client

	defer func() {
		Manager.unregister <- c
	}()

	// 연결 확인 메시지 전송
	welcomeMsg := models.WebSocketMessage{
		Type: models.MessageTypeSystemInfo,
		Data: map[string]interface{}{
			"message":      "웹 클라이언트 연결됨",
			"connected_at": time.Now().Format(time.RFC3339),
		},
		Timestamp: time.Now().UnixMilli(),
	}
	_ = c.WriteJSON(welcomeMsg)

	// ★ 신규: 연결 시 현재 모든 AGV 상태 전송
	if AGVMgr != nil {
		statuses := AGVMgr.GetAllStatuses()
		if len(statuses) > 0 {
			initialMsg := models.WebSocketMessage{
				Type: "agv_status_update",
				Data: map[string]interface{}{
					"agvs": statuses,
				},
				Timestamp: time.Now().UnixMilli(),
			}
			_ = c.WriteJSON(initialMsg)
			log.Printf("[Web] 초기 AGV 상태 전송: %d개", len(statuses))
		}
	}

	for {
		var msg models.WebSocketMessage
		err := c.ReadJSON(&msg)
		if err != nil {
			log.Printf("[Web] 메시지 읽기 오류: %v", err)
			break
		}

		// 타임스탬프 추가
		if msg.Timestamp == 0 {
			msg.Timestamp = time.Now().UnixMilli()
		}

		log.Printf("[Web] 메시지: %s - %+v", msg.Type, msg.Data)

		// 로깅
		go services.LogAGVEvent(msg, "", "web-user")

		// 채팅 메시지 처리 (LLM AnswerQuestion 호출)
		switch msg.Type {
		case models.MessageTypeChat:
			if chatData, ok := msg.Data.(map[string]interface{}); ok {
				if message, ok := chatData["message"].(string); ok {
					log.Printf("💬 사용자 질문: %s", message)

					go func() {
						if llmService == nil {
							log.Printf("⚠️  LLM 서비스가 초기화되지 않음")
							return
						}

						var status *models.AGVStatus
						if currentAGVStatus != nil {
							status = currentAGVStatus
						}

						response, err := llmService.AnswerQuestion(message, status)
						if err != nil {
							log.Printf("❌ LLM 응답 실패: %v", err)
							return
						}

						responseMsg := models.WebSocketMessage{
							Type: models.MessageTypeChatResponse,
							Data: models.ChatResponseData{
								Message:   response,
								Model:     llmService.Model,
								Timestamp: time.Now().UnixMilli(),
							},
							Timestamp: time.Now().UnixMilli(),
						}

						Manager.BroadcastMessage(responseMsg)
						if len(response) > 50 {
							log.Printf("✅ AI 응답 전송: %s...", response[:50])
						} else {
							log.Printf("✅ AI 응답 전송: %s", response)
						}
					}()

				}
			}

		case models.MessageTypeCommand,
			models.MessageTypeModeChange,
			models.MessageTypeEmergencyStop:
			// 명령 메시지는 AGV로 브로드캐스트
			Manager.broadcast <- msg

		default:
			log.Printf("[Web] 알 수 없는 메시지 타입: %s", msg.Type)
		}
	}
}
