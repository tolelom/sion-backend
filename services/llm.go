package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sion-backend/models"
	"time"
)

// LLMService - LLM API 통신 서비스
type LLMService struct {
	BaseURL string
	Model   string
}

// NewLLMServiceFromEnv - 환경 변수에서 Ollama 설정 읽기
func NewLLMServiceFromEnv() *LLMService {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "llama3.2"
	}

	log.Printf("✅ LLMService 초기화 (provider=ollama, baseURL=%s, model=%s)", baseURL, model)

	return &LLMService{
		BaseURL: baseURL,
		Model:   model,
	}
}

// AnswerQuestion - 사용자 질문에 답변 (WebSocket에서 호출)
func (s *LLMService) AnswerQuestion(question string, agvStatus *models.AGVStatus) (string, error) {
	systemPrompt := `당신은 AGV 로봇 "사이온"의 AI 해설자입니다.
리그오브레전드의 사이온 캐릭터처럼 용감하고 적극적인 톤으로 설명하세요.
사용자의 질문에 현재 AGV의 상태를 기반으로 명확하고 간결하게 답변하세요.
AGV 상태 정보가 없으면, 일반적인 사이온 컨셉에 맞게 대답하세요.
답변은 3-4문장 이내로 작성하세요.`

	var userPrompt string
	if agvStatus != nil {
		userPrompt = fmt.Sprintf(`[사용자 질문]
%s

[현재 AGV 상태]
- 위치: (%.1f, %.1f), 각도: %.1f°
- 모드: %s
- 상태: %s
- 배터리: %d%%
- 속도: %.1f m/s

`, question,
			agvStatus.Position.X,
			agvStatus.Position.Y,
			agvStatus.Position.Angle*180/math.Pi,
			agvStatus.Mode,
			agvStatus.State,
			agvStatus.Battery,
			agvStatus.Speed)

		if agvStatus.TargetEnemy != nil {
			userPrompt += fmt.Sprintf("- 현재 타겟: %s (체력 %d%%)\n",
				agvStatus.TargetEnemy.Name, agvStatus.TargetEnemy.HP)
		}

		if len(agvStatus.DetectedEnemies) > 0 {
			userPrompt += "\n[감지된 적]\n"
			for i, enemy := range agvStatus.DetectedEnemies {
				dist := calculateDistance(agvStatus.Position, enemy.Position)
				userPrompt += fmt.Sprintf("- 적 #%d: %s (체력 %d%%, 거리 %.1fm)\n",
					i+1, enemy.Name, enemy.HP, dist)
			}
		}

		userPrompt += "\n위 정보를 바탕으로 질문에 답변해주세요."
	} else {
		userPrompt = fmt.Sprintf(`[사용자 질문]
%s

AGV 상태 정보는 아직 없습니다. 사이온의 캐릭터성과 전투 스타일에 기반해 답변해주세요.`, question)
	}

	log.Printf("🤖 LLM 호출 (Ollama, model=%s): %s", s.Model, question)
	return s.callOllama(systemPrompt, userPrompt)
}

// ExplainEvent - AGV 이벤트 설명 생성
func (s *LLMService) ExplainEvent(eventType string, agvStatus *models.AGVStatus) (string, error) {
	systemPrompt := `당신은 AGV 로봇 "사이온"의 실시간 해설자입니다.
사이온의 행동을 마치 e스포츠 해설처럼 열정적이고 명확하게 설명하세요.
2-3문장으로 간결하게 작성하세요.`

	var userPrompt string

	switch eventType {
	case "target_change":
		if agvStatus != nil && agvStatus.TargetEnemy != nil {
			userPrompt = fmt.Sprintf(`[타겟 변경 이벤트 🎯]
현재 시각: %s
새로운 타겟: %s (체력 %d%%)
위치: (%.1f, %.1f)

왜 이 타겟을 선택했는지 설명해주세요.`,
				time.Now().Format("15:04:05"),
				agvStatus.TargetEnemy.Name,
				agvStatus.TargetEnemy.HP,
				agvStatus.Position.X,
				agvStatus.Position.Y)
		}

	case "charging":
		if agvStatus != nil {
			userPrompt = fmt.Sprintf(`[돌진 공격! ⚔️]
현재 시각: %s
사이온이 궁극기를 사용합니다!
위치: (%.1f, %.1f)
속도: %.1f m/s`,
				time.Now().Format("15:04:05"),
				agvStatus.Position.X,
				agvStatus.Position.Y,
				agvStatus.Speed)
		}

	default:
		userPrompt = fmt.Sprintf("[이벤트: %s] 현재 상황을 설명해주세요.", eventType)
	}

	if userPrompt == "" {
		userPrompt = fmt.Sprintf("[이벤트: %s] 현재 상황을 설명해주세요.", eventType)
	}

	return s.callOllama(systemPrompt, userPrompt)
}

func (s *LLMService) callOllama(systemPrompt, userPrompt string) (string, error) {
	start := time.Now() // ⏱️ 시작 시간

	fullPrompt := systemPrompt + "\n\n" + userPrompt

	body := map[string]interface{}{
		"model":  s.Model,
		"prompt": fullPrompt,
		"stream": false,
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ollama 요청 JSON 마샬링 실패: %v", err)
	}

	url := s.BaseURL + "/api/generate"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ollama 요청 생성 실패: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama 호출 실패: %v", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ollama 응답 읽기 실패: %v", err)
	}

	var result struct {
		Response string `json:"response"`
	}

	if err := json.Unmarshal(b, &result); err != nil {
		return "", fmt.Errorf("ollama 응답 파싱 실패: %v (body=%s)", err, string(b))
	}

	if result.Response == "" {
		return "", fmt.Errorf("ollama 응답이 비어있습니다: %s", string(b))
	}

	elapsed := time.Since(start) // ⏱️ 소요 시간
	log.Printf("⏱️ Ollama 응답 시간: %.2f초 (모델: %s)", elapsed.Seconds(), s.Model)

	return result.Response, nil
}

func calculateDistance(pos1, pos2 models.PositionData) float64 {
	dx := pos1.X - pos2.X
	dy := pos1.Y - pos2.Y
	return math.Sqrt(dx*dx + dy*dy)
}
