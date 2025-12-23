package handlers

import (
	"encoding/json"
	"log"
	"sync"

	"sion-backend/models"
)

// Manager - WebSocket 메시지 관리자
var Manager *MessageManager

// MessageManager - 메시지 관리 및 브로드캐스트
type MessageManager struct {
	broadcast chan []byte
	mu        sync.RWMutex
	clients   map[interface{}]bool
}

// NewMessageManager - 메시지 관리자 생성
func NewMessageManager() *MessageManager {
	return &MessageManager{
		broadcast: make(chan []byte, 256),
		clients:   make(map[interface{}]bool),
	}
}

// Start - 메시지 관리자 시작
func (m *MessageManager) Start() {
	log.Println("✅ MessageManager 시작")
	for msg := range m.broadcast {
		m.mu.RLock()
		for client := range m.clients {
			if conn, ok := client.(interface{ WriteMessage(int, []byte) error }); ok {
				if err := conn.WriteMessage(1, msg); err != nil {
					log.Printf("⚠️ 메시지 전송 오류: %v", err)
				}
			}
		}
		m.mu.RUnlock()
	}
}

// BroadcastMessage - 메시지 브로드캐스트
func (m *MessageManager) BroadcastMessage(msg models.WebSocketMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("❌ JSON 마샬링 오류: %v", err)
		return
	}

	select {
	case m.broadcast <- data:
	default:
		log.Println("⚠️ broadcast 채널 가득 참")
	}
}

// RegisterClient - 클라이언트 등록
func (m *MessageManager) RegisterClient(client interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[client] = true
}

// UnregisterClient - 클라이언트 제거
func (m *MessageManager) UnregisterClient(client interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, client)
}

// GetClientCount - 연결된 클라이언트 수 반환
func (m *MessageManager) GetClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// Init - Manager 초기화
func init() {
	if Manager == nil {
		Manager = NewMessageManager()
	}
}
