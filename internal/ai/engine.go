package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"dbterm/internal/config"
	"dbterm/internal/db"
)

// AIRequest represents a query assistance request
type AIRequest struct {
	Mode         AIMode
	UserPrompt   string
	CurrentSQL   string
	ErrorMessage string
	Driver       db.Driver
}

// AIResponse contains the generated output from the AI model
type AIResponse struct {
	GeneratedSQL string
	Explanation  string
	ModelUsed    string
	Duration     time.Duration
}

// Engine represents the inference engine interface
type Engine interface {
	Generate(ctx context.Context, req AIRequest) (*AIResponse, error)
	IsAvailable() bool
	GetModelInfo() string
}

// NewEngine initializes the appropriate AI engine based on configuration
func NewEngine(cfg config.AIConfig) Engine {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:11434"
	}
	if cfg.ModelName == "" {
		cfg.ModelName = DefaultModel.ID
	}
	return &localEngine{
		config:     cfg,
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}
}

type localEngine struct {
	config     config.AIConfig
	httpClient *http.Client
}

func (e *localEngine) IsAvailable() bool {
	return e.config.Enabled
}

func (e *localEngine) GetModelInfo() string {
	if IsModelInstalled(DefaultModel) {
		return fmt.Sprintf("Embedded Local (%s | NPU/CPU)", DefaultModel.Name)
	}
	return fmt.Sprintf("AI Engine (%s)", e.config.ModelName)
}

func (e *localEngine) Generate(ctx context.Context, req AIRequest) (*AIResponse, error) {
	startTime := time.Now()

	// 1. Build schema context
	schemaSummary, err := BuildSchemaContext(ctx, req.Driver, e.config.MaxSchemaTables)
	if err != nil {
		schemaSummary = &SchemaSummary{
			Database:   "ActiveDB",
			Dialect:    "SQL",
			DDLContext: "-- Schema unavailable\n",
		}
	}

	// 2. Build system and user prompts
	systemPrompt := BuildSystemPrompt(schemaSummary.Dialect, schemaSummary.DDLContext)
	userPrompt := BuildUserPrompt(req.Mode, req.UserPrompt, req.CurrentSQL, req.ErrorMessage)

	// 3. Perform Inference
	rawResponse, err := e.callInference(ctx, systemPrompt, userPrompt)
	if err != nil {
		// Use Embedded Schema-Aware Generator for seamless offline / zero-setup execution
		gen := NewEmbeddedSQLGenerator()
		sql, exp := gen.Generate(req, schemaSummary)
		return &AIResponse{
			GeneratedSQL: sql,
			Explanation:  exp,
			ModelUsed:    "Embedded Schema Generator (Offline)",
			Duration:     time.Since(startTime),
		}, nil
	}

	sqlCode, explanation := ExtractSQLAndExplanation(rawResponse)
	if sqlCode == "" && req.Mode == AIModeExplain {
		explanation = rawResponse
	}

	return &AIResponse{
		GeneratedSQL: sqlCode,
		Explanation:  explanation,
		ModelUsed:    e.config.ModelName,
		Duration:     time.Since(startTime),
	}, nil
}

func (e *localEngine) callInference(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// A. Try Ollama API format (/api/chat or /api/generate)
	if strings.Contains(e.config.Endpoint, "11434") || e.config.Provider == config.AIProviderOllama {
		resp, err := e.callOllama(ctx, systemPrompt, userPrompt)
		if err == nil && resp != "" {
			return resp, nil
		}
	}

	// B. Try OpenAI-compatible API format (/v1/chat/completions)
	resp, err := e.callOpenAICompat(ctx, systemPrompt, userPrompt)
	if err == nil && resp != "" {
		return resp, nil
	}

	return "", err
}

func (e *localEngine) callOllama(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	url := fmt.Sprintf("%s/api/chat", strings.TrimRight(e.config.Endpoint, "/"))
	payload := map[string]any{
		"model":  e.config.ModelName,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"options": map[string]any{
			"temperature": e.config.Temperature,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama HTTP %d", resp.StatusCode)
	}

	var resObj struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&resObj); err != nil {
		return "", err
	}
	return resObj.Message.Content, nil
}

func (e *localEngine) callOpenAICompat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	base := strings.TrimRight(e.config.Endpoint, "/")
	if !strings.HasSuffix(base, "/v1") && !strings.Contains(base, "/v1/") {
		base += "/v1"
	}
	url := fmt.Sprintf("%s/chat/completions", base)

	payload := map[string]any{
		"model":       e.config.ModelName,
		"temperature": e.config.Temperature,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.config.APIKey)
	} else if os.Getenv("OPENAI_API_KEY") != "" {
		req.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("inference server HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var resObj struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&resObj); err != nil {
		return "", err
	}

	if len(resObj.Choices) > 0 {
		return resObj.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no response choices returned")
}
