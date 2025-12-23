package main

import (
	"log"
	"sion-backend/handlers"
	"sion-backend/models"
	"sion-backend/services"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/websocket/v2"
	"github.com/joho/godotenv"
)

var agvSimulator *services.AGVSimulator
var agvMgr *handlers.AGVManager
var commentaryService *services.CommentaryService // 🆕 자동 중계 서비스

func setupAGVAPI(api fiber.Router, agvMgr *handlers.AGVManager) {
	agvAPI := api.Group("/agv")

	agvAPI.Get("/status/:id", func(c *fiber.Ctx) error {
		agvID := c.Params("id")
		info, err := agvMgr.GetStatus(agvID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"id":       info.ID,
				"position": info.Position,
				"mode":     info.Mode,
				"state":    info.State,
				"battery":  info.Battery,
				"speed":    info.Speed,
			},
		})
	})

	agvAPI.Get("/all", func(c *fiber.Ctx) error {
		statuses := agvMgr.GetAllStatuses()
		data := make([]interface{}, len(statuses))
		for i, info := range statuses {
			data[i] = fiber.Map{
				"id":       info.ID,
				"position": info.Position,
				"mode":     info.Mode,
				"state":    info.State,
				"battery":  info.Battery,
				"speed":    info.Speed,
			}
		}
		return c.JSON(fiber.Map{
			"success": true,
			"count":   len(data),
			"data":    data,
		})
	})

	agvAPI.Get("/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"data":    agvMgr.GetStatistics(),
		})
	})
}

// 🆕 자동 중계 API 설정
func setupCommentaryAPI(api fiber.Router) {
	commentaryAPI := api.Group("/commentary")

	// 자동 중계 상태 조회
	commentaryAPI.Get("/status", func(c *fiber.Ctx) error {
		if commentaryService == nil {
			return c.JSON(fiber.Map{
				"success": false,
				"error":   "Commentary service not initialized",
			})
		}
		return c.JSON(fiber.Map{
			"success": true,
			"enabled": true, // TODO: 실제 상태 반환
		})
	})

	// 자동 중계 활성화/비활성화
	commentaryAPI.Post("/toggle", func(c *fiber.Ctx) error {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid request body",
			})
		}

		if commentaryService != nil {
			commentaryService.SetEnabled(body.Enabled)
		}

		return c.JSON(fiber.Map{
			"success": true,
			"enabled": body.Enabled,
		})
	})

	// 수동 해설 트리거 (테스트용)
	commentaryAPI.Post("/trigger", func(c *fiber.Ctx) error {
		var body struct {
			EventType string                 `json:"event_type"`
			Data      map[string]interface{} `json:"data"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid request body",
			})
		}

		if commentaryService != nil {
			commentaryService.QueueEvent(body.EventType, body.Data)
		}

		return c.JSON(fiber.Map{
			"success":    true,
			"event_type": body.EventType,
		})
	})
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env file not found")
	}

	if err := services.InitDatabase(); err != nil {
		log.Fatalf("❌ DB init failed: %v", err)
	}

	services.InitLogging(50, 10*time.Second)
	defer services.StopLogging()

	handlers.InitLLMService()

	// 🆕 자동 중계 서비스 초기화
	llmService := services.NewLLMServiceFromEnv()
	commentaryService = services.NewCommentaryService(llmService, handlers.Manager.BroadcastMessage)
	commentaryService.Start()
	defer commentaryService.Stop()

	// 🆕 전역 변수로 설정 (다른 패키지에서 접근 가능)
	handlers.CommentarySvc = commentaryService

	log.Println("[Main] ✅ Commentary Service initialized")

	agvSimulator = services.NewAGVSimulator(handlers.Manager.BroadcastMessage)

	// 🆕 시뮬레이터에 자동 중계 서비스 연결
	agvSimulator.SetCommentaryService(commentaryService)

	agvMgr = handlers.NewAGVManager()
	handlers.AGVMgr = agvMgr
	log.Println("[Main] ✅ AGV Manager initialized")

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			count := agvMgr.CleanupOfflineAGVs(10 * time.Second)
			if count > 0 {
				log.Printf("[Main] Cleaned up %d offline AGVs", count)
			}
		}
	}()

	app := fiber.New()

	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173, http://localhost:3000, http://sion.tolelom.xyz",
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	go handlers.Manager.Start()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Sion WebSocket server running")
	})

	api := app.Group("/api")

	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":             "OK",
			"clients":            handlers.Manager.GetClientCount(),
			"connected_agvs":     agvMgr.GetConnectedAGVs(),
			"agv_count":          agvMgr.GetAGVCount(),
			"commentary_enabled": true, // 🆕
			"time":               time.Now().Format(time.RFC3339),
		})
	})

	api.Post("/chat", handlers.HandleChat)
	api.Post("/pathfinding", handlers.HandlePathfinding)

	logsAPI := api.Group("/logs")
	logsAPI.Get("/recent", handlers.HandleGetRecentLogs)
	logsAPI.Get("/range", handlers.HandleGetLogsByTimeRange)
	logsAPI.Get("/type", handlers.HandleGetLogsByEventType)
	logsAPI.Get("/stats", handlers.HandleGetLogStats)

	setupAGVAPI(api, agvMgr)
	setupCommentaryAPI(api) // 🆕 자동 중계 API

	simAPI := api.Group("/simulator")
	simAPI.Post("/start", func(c *fiber.Ctx) error {
		if agvSimulator.IsRunning {
			return c.Status(400).JSON(fiber.Map{"success": false})
		}
		agvSimulator.Start()
		return c.JSON(fiber.Map{"success": true})
	})

	simAPI.Post("/stop", func(c *fiber.Ctx) error {
		if !agvSimulator.IsRunning {
			return c.Status(400).JSON(fiber.Map{"success": false})
		}
		agvSimulator.Stop()
		return c.JSON(fiber.Map{"success": true})
	})

	simAPI.Get("/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "running": agvSimulator.IsRunning})
	})

	api.Post("/test/position", func(c *fiber.Ctx) error {
		testMsg := models.WebSocketMessage{
			Type: models.MessageTypePosition,
			Data: models.PositionData{
				X:         10.5,
				Y:         15.2,
				Angle:     1.57,
				Timestamp: float64(time.Now().UnixMilli()) / 1000.0,
			},
			Timestamp: time.Now().UnixMilli(),
		}
		handlers.Manager.BroadcastMessage(testMsg)
		services.LogAGVPosition("sion-001", testMsg.Data.(models.PositionData))
		return c.JSON(fiber.Map{"success": true})
	})

	api.Post("/test/status", func(c *fiber.Ctx) error {
		testMsg := models.WebSocketMessage{
			Type: models.MessageTypeStatus,
			Data: map[string]interface{}{
				"battery": 85,
				"speed":   1.5,
				"mode":    "auto",
				"state":   "moving",
			},
			Timestamp: time.Now().UnixMilli(),
		}
		handlers.Manager.BroadcastMessage(testMsg)
		services.LogWebSocketMessage("sion-001", testMsg)
		return c.JSON(fiber.Map{"success": true})
	})

	api.Post("/test/event", func(c *fiber.Ctx) error {
		testStatus := &models.AGVStatus{
			ID:   "sion-001",
			Name: "Sion",
			Position: models.PositionData{
				X:         10.5,
				Y:         15.2,
				Angle:     0.785,
				Timestamp: float64(time.Now().UnixMilli()) / 1000.0,
			},
			Mode:    models.ModeAuto,
			State:   models.StateCharging,
			Speed:   2.5,
			Battery: 85,
		}
		services.LogAGVStatus("sion-001", testStatus)
		handlers.ExplainAGVEvent("target_change", testStatus)
		return c.JSON(fiber.Map{"success": true})
	})

	// 🆕 자동 중계 테스트 엔드포인트
	api.Post("/test/commentary", func(c *fiber.Ctx) error {
		if commentaryService == nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   "Commentary service not initialized",
			})
		}

		// 테스트 이벤트 발생
		commentaryService.QueueEvent("target_found", map[string]interface{}{
			"enemy_name": "아리",
			"enemy_hp":   75,
			"distance":   5.5,
		})

		return c.JSON(fiber.Map{
			"success": true,
			"message": "Commentary test event queued",
		})
	})

	app.Use("/websocket", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/websocket/agv", websocket.New(handlers.HandleAGVWebSocket))
	app.Get("/websocket/web", websocket.New(handlers.HandleWebWebSocket))

	log.Println("================================================")
	log.Println("🚀 Sion Backend Server")
	log.Println("================================================")
	log.Println("📡 WebSocket AGV: ws://localhost:3000/websocket/agv")
	log.Println("📡 WebSocket Web: ws://localhost:3000/websocket/web")
	log.Println("🔍 AGV Status:    GET /api/agv/all")
	log.Println("🎙️ Commentary:    POST /api/commentary/toggle")
	log.Println("💾 Health Check:  GET /api/health")
	log.Println("================================================")

	log.Fatal(app.Listen(":3000"))
}
