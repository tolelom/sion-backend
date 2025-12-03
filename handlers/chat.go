package handlers

import (
	"log"
	"sion-backend/models"
	"sion-backend/services"
	"time"

	"github.com/gofiber/fiber/v2"
)

var llmService *services.LLMService

// 임시 AGV 상태 (실제로는 전역 상태 관리 필요)
var currentAGVStatus *models.AGVStatus

// InitLLMService - LLM 서비스 초기화
func InitLLMService() {
	llmService = services.NewLLMServiceFromEnv()
	if llmService == nil {
		log.Println("⚠️  LLM 서비스 초기화 실패")
		return
	}
	log.Printf("✅ LLM 서비스 초기화 완료 (Ollama, model=%s)", llmService.Model)
}

// HandleChat - 채팅 메시지 처리 (HTTP POST)
func HandleChat(c *fiber.Ctx) error {
	var chatData models.ChatMessageData
	if err := c.BodyParser(&chatData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "잘못된 요청 형식",
		})
	}

	if llmService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "LLM 서비스가 초기화되지 않았습니다",
		})
	}

	log.Printf("💬 채팅 수신: %s", chatData.Message)

	// LLM에 질문
	response, err := llmService.AnswerQuestion(chatData.Message, currentAGVStatus)
	if err != nil {
		log.Printf("❌ LLM 오류: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "AI 응답 생성 실패: " + err.Error(),
		})
	}

	// WebSocket으로 브로드캐스트
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

	log.Printf("✅ AI 응답 전송: %s", response)

	return c.JSON(fiber.Map{
		"success":  true,
		"response": response,
	})
}

// ExplainAGVEvent - AGV 이벤트 자동 설명 (내부 호출용)
func ExplainAGVEvent(eventType string, agvStatus *models.AGVStatus) {
	if llmService == nil {
		return
	}

	// 비동기로 처리
	go func() {
		explanation, err := llmService.ExplainEvent(eventType, agvStatus)
		if err != nil {
			log.Printf("❌ 이벤트 설명 생성 실패: %v", err)
			return
		}

		// WebSocket으로 브로드캐스트
		eventMsg := models.WebSocketMessage{
			Type: models.MessageTypeAGVEvent,
			Data: models.AGVEventData{
				EventType:   eventType,
				Explanation: explanation,
				Position:    agvStatus.Position,
				Timestamp:   time.Now().UnixMilli(),
			},
			Timestamp: time.Now().UnixMilli(),
		}

		Manager.BroadcastMessage(eventMsg)
		log.Printf("📢 이벤트 설명 전송 [%s]: %s", eventType, explanation)
	}()
}

// UpdateAGVStatus - AGV 상태 업데이트 (다른 핸들러에서 호출)
func UpdateAGVStatus(status *models.AGVStatus) {
	currentAGVStatus = status
}
