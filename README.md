# 🚀 Sion AGV Backend

리그오브레전드 사이온의 궁극기 "멈출 수 없는 맹공"을 구현한 AGV 프로젝트의 백엔드 서버입니다.

## 📌 주요 기능

- **실시간 WebSocket 통신** - AGV와 웹 클라이언트 간 양방향 통신
- **A* 경로 탐색** - 장애물 회피 최적 경로 생성
- **LLM 통합** - OpenAI GPT를 활용한 AGV 행동 실시간 해설
- **채팅 시스템** - 사용자 질문에 AI 답변
- **RESTful API** - 경로 탐색, 채팅, 테스트 엔드포인트

## 🛠️ 기술 스택

- **Language**: Go 1.21+
- **Framework**: Fiber v2
- **WebSocket**: gofiber/websocket
- **AI**: OpenAI GPT-4o-mini
- **Architecture**: Clean Architecture (handlers, models, services)

## 📂 프로젝트 구조

```
sion-backend/
├── main.go                 # 서버 엔트리포인트
├── .env                    # 환경 변수 (OpenAI API Key)
├── go.mod                  # Go 의존성 관리
├── go.sum
├── algorithms/
│   └── astar.go           # A* 경로 탐색 알고리즘
├── handlers/
│   ├── chat.go            # 채팅 및 LLM 핸들러
│   ├── pathfinding.go     # 경로 탐색 핸들러
│   └── websocket.go       # WebSocket 연결 관리
├── models/
│   ├── agv.go             # AGV 상태 모델
│   ├── enemy.go           # 적(타겟) 모델
│   ├── map.go             # 맵 데이터 모델
│   └── message.go         # WebSocket 메시지 모델
└── services/
    └── llm.go             # OpenAI API 통신 서비스
```

## 🚀 시작하기

### 1. 사전 요구사항

- Go 1.21 이상
- OpenAI API Key ([https://platform.openai.com/api-keys](https://platform.openai.com/api-keys))

### 2. 설치

```bash
# 저장소 클론
git clone https://github.com/tolelom/sion-backend.git
cd sion-backend

# 의존성 설치
go mod download
```

### 3. 환경 변수 설정

`.env` 파일 생성:

```env
OPENAI_API_KEY=your_openai_api_key_here
```

### 4. 실행

```bash
go run main.go
```

서버가 `http://localhost:3000`에서 실행됩니다.

## 📡 API 엔드포인트

### REST API

| Method | Endpoint | 설명 |
|--------|----------|------|
| `GET` | `/` | 서버 상태 확인 |
| `GET` | `/api/health` | 헬스 체크 |
| `POST` | `/api/chat` | 채팅 메시지 전송 |
| `POST` | `/api/pathfinding` | A* 경로 탐색 요청 |
| `POST` | `/api/test/position` | 테스트용 위치 데이터 전송 |
| `POST` | `/api/test/status` | 테스트용 상태 데이터 전송 |
| `POST` | `/api/test/event` | 테스트용 AGV 이벤트 트리거 |

### WebSocket

| Endpoint | 설명 |
|----------|------|
| `ws://localhost:3000/websocket/agv` | AGV 연결용 |
| `ws://localhost:3000/websocket/web` | 웹 클라이언트 연결용 |

## 💬 채팅 API 사용 예제

```bash
curl -X POST http://localhost:3000/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "현재 상황을 설명해줘",
    "timestamp": 1234567890
  }'
```

## 🗺️ 경로 탐색 API 사용 예제

```bash
curl -X POST http://localhost:3000/api/pathfinding \
  -H "Content-Type: application/json" \
  -d '{
    "start": {"x": 0, "y": 0},
    "goal": {"x": 10, "y": 10},
    "obstacles": [
      {"x": 5, "y": 5},
      {"x": 5, "y": 6}
    ],
    "map_width": 20,
    "map_height": 20
  }'
```

## 📨 WebSocket 메시지 타입

### AGV → Server → Web

- `position` - AGV 위치 업데이트
- `status` - AGV 상태 업데이트
- `log` - 행동 로그
- `target_found` - 적 발견
- `path_update` - 경로 업데이트

### Web → Server → AGV

- `command` - 이동/정지 명령
- `mode_change` - 자동/수동 모드 전환
- `emergency_stop` - 긴급 정지

### Server → Web

- `chat_response` - AI 채팅 응답
- `agv_event` - AGV 이벤트 설명
- `llm_explanation` - AI 설명
- `system_info` - 시스템 정보

## 🧪 테스트

### 이벤트 설명 테스트

```bash
curl -X POST http://localhost:3000/api/test/event
```

이 명령은 타겟 변경 이벤트를 시뮬레이션하고 LLM이 생성한 설명을 WebSocket으로 전송합니다.

## 🔧 개발

### 코드 포맷팅

```bash
go fmt ./...
```

### 빌드

```bash
go build -o sion-backend
```

### 실행 파일 실행

```bash
./sion-backend
```

## 📝 주요 모델

### AGVStatus

```go
type AGVStatus struct {
    ID          string
    Name        string
    Position    PositionData
    Mode        string  // "auto" | "manual"
    State       string  // "idle" | "moving" | "charging"
    Speed       float64
    Battery     int
    TargetEnemy *Enemy
    DetectedEnemies []Enemy
}
```

### Enemy

```go
type Enemy struct {
    ID       string
    Name     string
    HP       int
    Position PositionData
    Distance float64
}
```

## 🤝 기여

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📄 라이선스

This project is licensed under the MIT License.

## 👥 팀원

- **김성민** - Backend Developer

## 🔗 관련 링크

- [Frontend Repository](https://github.com/tolelom/sion-frontend)
