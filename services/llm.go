package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// LLMService - LLM 서비스 (Ollama 또는 OpenAI 호환)
type LLMService struct {
	BaseURL    string
	Model      string
	APIKey     string // OpenAI 호환 API용
	HTTPClient *http.Client
}

// OllamaRequest - Ollama API 요청
type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *OllamaOptions  `json:"options,omitempty"`
}

// OllamaMessage - Ollama 메시지
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaOptions - Ollama 옵션
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"num_predict,omitempty"`
}

// OllamaResponse - Ollama API 응답
type OllamaResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// NewLLMService - LLM 서비스 생성
func NewLLMService(baseURL, model string) *LLMService {
	return &LLMService{
		BaseURL: baseURL,
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// NewLLMServiceFromEnv - 환경변수에서 LLM 서비스 생성
func NewLLMServiceFromEnv() *LLMService {
	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434" // 기본 Ollama 주소
	}

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "llama3.2" // 기본 모델
	}

	apiKey := os.Getenv("LLM_API_KEY") // OpenAI 호환 API용

	log.Printf("🤖 LLM Service: %s (model: %s)", baseURL, model)

	return &LLMService{
		BaseURL: baseURL,
		Model:   model,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// callOllama - Ollama API 호출
func (ls *LLMService) callOllama(systemPrompt, userPrompt string) (string, error) {
	messages := []OllamaMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	reqBody := OllamaRequest{
		Model:    ls.Model,
		Messages: messages,
		Stream:   false,
		Options: &OllamaOptions{
			Temperature: 0.7,
			MaxTokens:   200,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("JSON 마샬링 실패: %w", err)
	}

	url := fmt.Sprintf("%s/api/chat", ls.BaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("요청 생성 실패: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if ls.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+ls.APIKey)
	}

	resp, err := ls.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM 응답 오류 (%d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("응답 파싱 실패: %w", err)
	}

	return ollamaResp.Message.Content, nil
}

// GenerateCommentary - 해설 생성 (외부에서 호출 가능)
func (ls *LLMService) GenerateCommentary(eventType, context string) (string, error) {
	systemPrompt := `당신은 AGV 로봇 "사이온"의 실시간 e스포츠 해설자입니다.

🎙️ 해설 스타일:
- 열정적이고 흥분된 톤
- 짧고 임팩트 있는 문장 (2-3문장)
- 리그오브레전드 사이온 캐릭터의 특성 반영
- 한국어 e스포츠 중계 스타일
- 이모지 적절히 사용

⚠️ 주의사항:
- 반드시 2-3문장으로 짧게
- 기술적인 용어보다 재미있는 표현 사용`

	return ls.callOllama(systemPrompt, context)
}

// Chat - 일반 채팅 응답
func (ls *LLMService) Chat(userMessage string) (string, error) {
	systemPrompt := `당신은 AGV 로봇 "사이온"입니다.
리그오브레전드의 사이온 캐릭터처럼 강인하고 불굴의 의지를 가진 성격으로 대화합니다.
짧고 간결하게 답변하세요.`

	return ls.callOllama(systemPrompt, userMessage)
}

// IsAvailable - LLM 서비스 사용 가능 여부 확인
func (ls *LLMService) IsAvailable() bool {
	url := fmt.Sprintf("%s/api/tags", ls.BaseURL)
	resp, err := ls.HTTPClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
