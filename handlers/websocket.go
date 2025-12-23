package handlers

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

// 메시지 타입 정의
const (
	MsgTypeInit          = "init"
	MsgTypeInitAck       = "init_ack"
	MsgTypePosition      = "position"
	MsgTypeStatus        = "status"
	MsgTypeLog           = "log"
	MsgTypeHeartbeat     = "heartbeat"
	MsgTypeHeartbeatAck  = "heartbeat_ack"
	MsgTypeCommand       = "command"
	MsgTypeMapData       = "map_data"
	MsgTypeModeChange    = "mode_change"
	MsgTypeEmergencyStop = "emergency_stop"
	MsgTypeConnStatus    = "connection_status"
)

// WebSocket 메시지 구조체
type WSMessage struct {
	Type      string                 `json:"type"`
	AGVID     string                 `json:"agv_id,omitempty"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// 위치 데이터
type PositionData struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Heading    float64 `json:"heading"`
	Confidence float64 `json:"confidence"`
}

// 클라이언트 정보
type Client struct {
	Conn       *websocket.Conn
	ClientType string // "agv" or "web"
	AGVID      string
	LastSeen   time.Time
	Position   PositionData
	mu         sync.Mutex
}

// 멜룄 관리자
type Hub struct {
	agvClients map[string]*Client
	webClients map[*websocket.Conn]*Client
	broadcast  chan []byte
	toAGV      chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// 맥 데이터 (임시 저장)
type MapData struct {
	Width         int          `json:"width"`
	Height        int          `json:"height"`
	CellSize      float64      `json:"cell_size"`
	Obstacles     [][]int      `json:"obstacles"`
	StartPosition PositionData `json:"start_position"`
}

var (
	hub        *Hub
	currentMap *MapData
)

// 멜룄 초기화
func init() {
	hub = &Hub{
		agvClients: make(map[string]*Client),
		webClients: make(map[*websocket.Conn]*Client),
		broadcast:  make(chan []byte, 256),
		toAGV:      make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}

	// 기본 맥 데이터 초기화
	currentMap = &MapData{
		Width:         60,
		Height:        60,
		CellSize:      1.0,
		Obstacles:     [][]int{},
		StartPosition: PositionData{X: 0, Y: 0, Heading: 0},
	}
}

// 멜룄 시작
func StartHub() {
	go hub.run()
	go hub.monitorConnections()
	log.Println("✅ WebSocket Hub 시작됨")
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if client.ClientType == "agv" {
				h.agvClients[client.AGVID] = client
				log.Printf("🤖 AGV 연결: %s", client.AGVID)
			} else {
				h.webClients[client.Conn] = client
				log.Println("🌐 Web 클라이언트 연결")
			}
			h.mu.Unlock()

			// 연결 상태 브로드캐스트
			h.broadcastConnectionStatus()

		case client := <-h.unregister:
			h.mu.Lock()
			if client.ClientType == "agv" {
				delete(h.agvClients, client.AGVID)
				log.Printf("🤖 AGV 연결 해제: %s", client.AGVID)
			} else {
				delete(h.webClients, client.Conn)
				log.Println("🌐 Web 클라이언트 연결 해제")
			}
			h.mu.Unlock()

			h.broadcastConnectionStatus()

		case message := <-h.broadcast:
			// Web 클라이언트들에게 브로드캐스트
			h.mu.RLock()
			for _, client := range h.webClients {
				client.mu.Lock()
				err := client.Conn.WriteMessage(websocket.TextMessage, message)
				client.mu.Unlock()
				if err != nil {
					log.Printf("⚠️ Web 클라이언트 전송 오류: %v", err)
				}
			}
			h.mu.RUnlock()

		case message := <-h.toAGV:
			// AGV들에게 전송
			h.mu.RLock()
			for agvID, client := range h.agvClients {
				client.mu.Lock()
				err := client.Conn.WriteMessage(websocket.TextMessage, message)
				client.mu.Unlock()
				if err != nil {
					log.Printf("⚠️ AGV %s 전송 오류: %v", agvID, err)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// 연결 상태 모니터링
func (h *Hub) monitorConnections() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		h.mu.RLock()
		for agvID, client := range h.agvClients {
			if time.Since(client.LastSeen) > 60*time.Second {
				log.Printf("⚠️ AGV %s Heartbeat 타임아웃 (마지막: %v)",
					agvID, client.LastSeen.Format("15:04:05"))
			}
		}

		// 현재 연결 상태 로그
		log.Printf("📋 연결 상태: AGV=%d, Web=%d",
			len(h.agvClients), len(h.webClients))
		h.mu.RUnlock()
	}
}

// 연결 상태 브로드캐스트
func (h *Hub) broadcastConnectionStatus() {
	h.mu.RLock()
	agvConnected := len(h.agvClients) > 0
	agvList := make([]map[string]interface{}, 0)

	for agvID, client := range h.agvClients {
		agvList = append(agvList, map[string]interface{}{
			"id":        agvID,
			"last_seen": client.LastSeen.UnixMilli(),
			"position": map[string]float64{
				"x": client.Position.X,
				"y": client.Position.Y,
			},
		})
	}
	h.mu.RUnlock()

	msg := WSMessage{
		Type:      MsgTypeConnStatus,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"agv_connected": agvConnected,
			"agv_count":     len(agvList),
			"agv_list":      agvList,
			"web_count":     len(h.webClients),
		},
	}

	data, _ := json.Marshal(msg)

	// 비동기 전송
	select {
	case h.broadcast <- data:
	default:
		log.Println("⚠️ broadcast 채널 가득 찥")
	}
}

// AGV WebSocket 핸들러
func HandleAGVWebSocket(c *websocket.Conn) {
	client := &Client{
		Conn:       c,
		ClientType: "agv",
		LastSeen:   time.Now(),
	}

	defer func() {
		hub.unregister <- client
		c.Close()
	}()

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("AGV 연결 정상 종료")
			} else {
				log.Printf("❌ AGV 메시지 수신 오류: %v", err)
			}
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			log.Printf("❌ JSON 파싱 오류: %v", err)
			continue
		}

		client.LastSeen = time.Now()

		switch wsMsg.Type {
		case MsgTypeInit:
			client.AGVID = wsMsg.AGVID
			if client.AGVID == "" {
				client.AGVID = "agv-unknown"
			}
			hub.register <- client

			// 초기화 응답
			ackMsg := WSMessage{
				Type:      MsgTypeInitAck,
				Timestamp: time.Now().UnixMilli(),
				Data: map[string]interface{}{
					"status":      "connected",
					"server_time": time.Now().UnixMilli(),
					"agv_id":      client.AGVID,
				},
			}
			data, _ := json.Marshal(ackMsg)
			client.mu.Lock()
			c.WriteMessage(websocket.TextMessage, data)
			client.mu.Unlock()

			log.Printf("✅ AGV %s 초기화 완료", client.AGVID)

			// 맥 데이터 전송
			sendMapData(client)

		case MsgTypePosition:
			// 위치 데이터 파싱 및 저장
			if data, ok := wsMsg.Data["x"].(float64); ok {
				client.Position.X = data
			}
			if data, ok := wsMsg.Data["y"].(float64); ok {
				client.Position.Y = data
			}
			if data, ok := wsMsg.Data["heading"].(float64); ok {
				client.Position.Heading = data
			}

			// Web 클라이언트에 브로드캐스트
			hub.broadcast <- msg

		case MsgTypeStatus:
			// 상태 데이터 Web 클라이언트에 브로드캐스트
			hub.broadcast <- msg
			log.Printf("📋 AGV %s 상태: %v", client.AGVID, wsMsg.Data)

		case MsgTypeLog:
			// 로그 데이터 Web 클라이언트에 브로드캐스트
			hub.broadcast <- msg

			// 로그 레벨에 따른 출력
			level, _ := wsMsg.Data["level"].(string)
			event, _ := wsMsg.Data["event"].(string)
			message, _ := wsMsg.Data["message"].(string)

			switch level {
			case "warning":
				log.Printf("⚠️ [%s] %s: %s", client.AGVID, event, message)
			case "error":
				log.Printf("❌ [%s] %s: %s", client.AGVID, event, message)
			default:
				log.Printf("📏 [%s] %s: %s", client.AGVID, event, message)
			}

			// TODO: DB에 로그 저장

		case MsgTypeHeartbeat:
			// Heartbeat 응답
			ackMsg := WSMessage{
				Type:      MsgTypeHeartbeatAck,
				Timestamp: time.Now().UnixMilli(),
				Data:      map[string]interface{}{},
			}
			data, _ := json.Marshal(ackMsg)
			client.mu.Lock()
			c.WriteMessage(websocket.TextMessage, data)
			client.mu.Unlock()

		default:
			log.Printf("⚠️ 알 수 없는 메시지 타입: %s", wsMsg.Type)
		}
	}
}

// 맥 데이터 전송
func sendMapData(client *Client) {
	mapMsg := WSMessage{
		Type:      MsgTypeMapData,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"width":          currentMap.Width,
			"height":         currentMap.Height,
			"cell_size":      currentMap.CellSize,
			"obstacles":      currentMap.Obstacles,
			"start_position": currentMap.StartPosition,
		},
	}
	data, _ := json.Marshal(mapMsg)
	client.mu.Lock()
	client.Conn.WriteMessage(websocket.TextMessage, data)
	client.mu.Unlock()
	log.Printf("🗷️ 맥 데이터 전송: %s", client.AGVID)
}

// Web WebSocket 핸들러
func HandleWebWebSocket(c *websocket.Conn) {
	client := &Client{
		Conn:       c,
		ClientType: "web",
		LastSeen:   time.Now(),
	}

	hub.register <- client

	defer func() {
		hub.unregister <- client
		c.Close()
	}()

	// 연결 시 현재 연결 상태 전송
	hub.broadcastConnectionStatus()

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("Web 클라이언트 연결 정상 종료")
			} else {
				log.Printf("❌ Web 메시지 수신 오류: %v", err)
			}
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			log.Printf("❌ JSON 파싱 오류: %v", err)
			continue
		}

		client.LastSeen = time.Now()

		switch wsMsg.Type {
		case MsgTypeCommand:
			// AGV에 명령 전달
			log.Printf("🅶 명령 전달: %v", wsMsg.Data)
			hub.toAGV <- msg

		case MsgTypeModeChange:
			// 모드 변경 명령 전달
			log.Printf("🔄 모드 변경 명령: %v", wsMsg.Data)
			hub.toAGV <- msg

		case MsgTypeEmergencyStop:
			// 긴급 정지 명령 전달
			log.Printf("🛱 긴급 정지 명령!")
			hub.toAGV <- msg

		case "get_status":
			// 현재 상태 요청
			hub.broadcastConnectionStatus()

		default:
			log.Printf("⚠️ Web 클라이언트 알 수 없는 메시지: %s", wsMsg.Type)
		}
	}
}

// 맥 데이터 업데이트 (외부에서 호출)
func UpdateMapData(mapData *MapData) {
	currentMap = mapData
	log.Printf("🗷️ 맥 데이터 업데이트: %dx%d", mapData.Width, mapData.Height)

	// 연결된 모든 AGV에 맥 데이터 전송
	hub.mu.RLock()
	for _, client := range hub.agvClients {
		sendMapData(client)
	}
	hub.mu.RUnlock()
}

// AGV에 명령 전송 (외부에서 호출)
func SendCommandToAGV(action string, target map[string]float64) {
	msg := WSMessage{
		Type:      MsgTypeCommand,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"action": action,
			"target": target,
		},
	}
	data, _ := json.Marshal(msg)
	hub.toAGV <- data
}

// 연결된 AGV 모니 반환
func GetConnectedAGVs() []string {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	agvList := make([]string, 0, len(hub.agvClients))
	for agvID := range hub.agvClients {
		agvList = append(agvList, agvID)
	}
	return agvList
}

// AGV 연결 상태 확인
func IsAGVConnected(agvID string) bool {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	_, exists := hub.agvClients[agvID]
	return exists
}
