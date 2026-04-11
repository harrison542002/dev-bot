package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/harrison542002/dev-bot/internal/config"
)

type geminiClient struct {
	apiKey string
	model  string
}

func newGeminiClient(cfg *config.GeminiConfig) Client {
	return &geminiClient{apiKey: cfg.APIKey, model: cfg.Model}
}

func (c *geminiClient) ProviderName() string { return "Gemini" }

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int64 `json:"promptTokenCount"`
		CandidatesTokenCount int64 `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *geminiClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.model, c.apiKey,
	)

	reqBody := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: system}},
		},
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: user}}},
		},
		GenerationConfig: geminiGenConfig{MaxOutputTokens: maxTokens},
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", nil, fmt.Errorf("build gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("gemini HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result geminiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", nil, fmt.Errorf("gemini response parse: %w", err)
	}
	if result.Error != nil {
		return "", nil, fmt.Errorf("gemini error: %s", result.Error.Message)
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", nil, fmt.Errorf("gemini returned empty response")
	}

	var usage *Usage
	if result.UsageMetadata != nil {
		usage = &Usage{
			InputTokens:  result.UsageMetadata.PromptTokenCount,
			OutputTokens: result.UsageMetadata.CandidatesTokenCount,
		}
	}
	return result.Candidates[0].Content.Parts[0].Text, usage, nil
}
