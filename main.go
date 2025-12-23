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
var mapGenerator *services.MapGenerator          // 🗺️ Map Generator

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

// 🗺️ Map API 설정
func setupMapAPI(api fiber.Router) {
	mapAPI := api.Group("/map")

	// 현재 맵 조회
	mapAPI.Get("/current", func(c *fiber.Ctx) error {
		activeMap := mapGenerator.GetActiveMap()
		if activeMap == nil {
			return c.Status(404).JSON(fiber.Map{
				"success": false,
				"error":   "No active map",
			})
		}
		return c.JSON(fiber.Map{
			"success": true,
			"data":    activeMap,
		})
	})

	// 목표 지점 설정
	mapAPI.Post("/goal", func(c *fiber.Ctx) error {
		var req struct {
			X      float64 `json:"x"`
			Y      float64 `json:"y"`
			Z      float64 `json:"z"`
			Radius float64 `json:"radius"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid request body",
			})
		}

		// 기본 반경 설정
		if req.Radius == 0 {
			req.Radius = 0.5
		}

		position := models.Position{
			X: req.X,
			Y: req.Y,
			Z: req.Z,
		}

		// 위치 유효성 검사
		if !mapGenerator.IsPositionValid(position) {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid position (out of bounds or inside obstacle)",
			})
		}

		// 목표 추가
		goal, err := mapGenerator.AddGoal(position, req.Radius)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}

		// 🗺️ WebSocket으로 목표 설정 브로드캐스트
		goalSetMsg := models.WebSocketMessage{
			Type: models.MessageTypeGoalSet,
			Data: models.GoalSetData{
				GoalID:   goal.ID,
				Position: position,
				Radius:   req.Radius,
			},
			Timestamp: time.Now().UnixMilli(),
		}
		handlers.Manager.BroadcastMessage(goalSetMsg)

		// 📡 AGV에 이동 명령 전송
		agvCommandMsg := models.WebSocketMessage{
			Type: models.MessageTypeAGVCommand,
			Data: models.AGVCommandMessage{
				AGVID:     "sion-001", // TODO: 실제 AGV ID 관리
				Command:   "move_to",
				TargetPos: position,
				Timestamp: time.Now().UnixMilli(),
			},
			Timestamp: time.Now().UnixMilli(),
		}
		handlers.Manager.BroadcastMessage(agvCommandMsg)

		return c.JSON(fiber.Map{
			"success": true,
			"goal":    goal,
		})
	})

	// 연결 상태 및 맵 상태 확인
	mapAPI.Get("/status", func(c *fiber.Ctx) error {
		activeMap := mapGenerator.GetActiveMap()
		return c.JSON(fiber.Map{
			"success":        true,
			"agv_count":      agvMgr.GetAGVCount(),
			"client_count":   handlers.Manager.GetClientCount(),
			"map_generated":  activeMap != nil,
			"system_ready":   activeMap != nil && agvMgr.GetAGVCount() > 0,
		})
	})

	// 수동 맵 생성 (테스트용)
	mapAPI.Post("/generate", func(c *fiber.Ctx) error {
		var req struct {
			Width    float64 `json:"width"`
			Height   float64 `json:"height"`
			CellSize float64 `json:"cell_size"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid request body",
			})
		}

		// 기본값 설정
		if req.Width == 0 {
			req.Width = 20.0
		}
		if req.Height == 0 {
			req.Height = 20.0
		}
		if req.CellSize == 0 {
			req.CellSize = 0.5
		}

		mapGrid := mapGenerator.GenerateMap(req.Width, req.Height, req.CellSize)

		// 📡 모든 클라이언트에 맵 브로드캐스트
		broadcastMapToClients()

		return c.JSON(fiber.Map{
			"success": true,
			"map":     mapGrid,
		})
	})
}

// 📡 맵을 모든 클라이언트에 브로드캐스트
func broadcastMapToClients() {
	mapMsg := mapGenerator.GetMapGridMessage()
	if mapMsg == nil {
		log.Println("[Map] No active map to broadcast")
		return
	}

	broadcastMsg := models.WebSocketMessage{
		Type:      models.MessageTypeMapGrid,
		Data:      mapMsg,
		Timestamp: time.Now().UnixMilli(),
	}

	handlers.Manager.BroadcastMessage(broadcastMsg)
	log.Printf("[Map] ✅ Broadcasted map (ID: %s) to all clients\n", mapMsg.MapID)
}

// 🤖 시스템 준비 확인 및 자동 맵 생성
func checkSystemReadyAndGenerateMap() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		mapGenerated := false

		for range ticker.C {
			// 이미 맵이 생성되었으면 스킵
			if mapGenerated {
				continue
			}

			// 조건: AGV 최소 1개 + 클라이언트 최소 1개
			agvCount := agvMgr.GetAGVCount()
			clientCount := handlers.Manager.GetClientCount()

			if agvCount > 0 && clientCount > 0 {
				log.Printf("[Map] 🎯 System Ready! AGV: %d, Clients: %d\n", agvCount, clientCount)

				// 맵 생성
				mapGenerator.GenerateMap(20.0, 20.0, 0.5)
				log.Println("[Map] 🗺️  Map generated successfully")

				// 모든 클라이언트에 브로드캐스트
				broadcastMapToClients()

				// System Ready 알림
				readyMsg := models.WebSocketMessage{
					Type: models.MessageTypeSystemReady,
					Data: models.SystemReadyData{
						AGVCount:     agvCount,
						ClientCount:  clientCount,
						MapGenerated: true,
					},
					Timestamp: time.Now().UnixMilli(),
				}
				handlers.Manager.BroadcastMessage(readyMsg)

				mapGenerated = true
			}
		}
	}()
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

	// 🗺️ Map Generator 초기화
	mapGenerator = services.NewMapGenerator()
	log.Println("[Main] ✅ Map Generator initialized")

	agvSimulator = services.NewAGVSimulator(handlers.Manager.BroadcastMessage)

	// 🆕 시뮬레이터에 자동 중계 서비스 연결
	agvSimulator.SetCommentaryService(commentaryService)

	agvMgr = handlers.NewAGVManager()
	handlers.AGVMgr = agvMgr
	log.Println("[Main] ✅ AGV Manager initialized")

	// 🤖 시스템 준비 확인 및 자동 맵 생성
	checkSystemReadyAndGenerateMap()

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
		activeMap := mapGenerator.GetActiveMap()
		return c.JSON(fiber.Map{
			"status":             "OK",
			"clients":            handlers.Manager.GetClientCount(),
			"connected_agvs":     agvMgr.GetConnectedAGVs(),
			"agv_count":          agvMgr.GetAGVCount(),
			"commentary_enabled": true,
			"map_generated":      activeMap != nil,
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
	setupMapAPI(api)        // 🗺️ Map API

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
	log.Println("🗺️  Map Status:    GET /api/map/status")
	log.Println("🎯 Set Goal:      POST /api/map/goal")
	log.Println("🎙️  Commentary:    POST /api/commentary/toggle")
	log.Println("💾 Health Check:  GET /api/health")
	log.Println("================================================")

	log.Fatal(app.Listen(":3000"))
}
