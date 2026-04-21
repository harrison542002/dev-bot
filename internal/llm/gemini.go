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

type GeminiClient struct {
	apiKey string
	model  string
}

func NewGeminiClient(cfg *config.GeminiConfig) Client {
	return &GeminiClient{apiKey: cfg.APIKey, model: cfg.Model}
}

func (c *GeminiClient) ProviderName() string { return "Gemini" }

// ── simple completion ──────────────────────────────────────────────────────────

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
	Text             string          `json:"text,omitempty"`
	FunctionCall     *geminiFuncCall `json:"functionCall,omitempty"`
	FunctionResponse *geminiFuncResp `json:"functionResponse,omitempty"`
}

type geminiFuncCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiFuncResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiGenConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
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

func (c *GeminiClient) Complete(ctx context.Context, system, user string, maxTokens int) (string, *Usage, error) {
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

// ── tool-use completion ────────────────────────────────────────────────────────

type geminiToolRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	Tools             []geminiTool    `json:"tools,omitempty"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

type geminiFuncDecl struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Parameters  geminiToolSchema `json:"parameters"`
}

type geminiToolSchema struct {
	Type       string                    `json:"type"` // "OBJECT"
	Properties map[string]geminiToolProp `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

type geminiToolProp struct {
	Type        string `json:"type"` // "STRING", "BOOLEAN", etc.
	Description string `json:"description"`
}

func (c *GeminiClient) CompleteWithTools(ctx context.Context, system string, messages []Message, tools []Tool, maxTokens int) (Message, *Usage, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.model, c.apiKey,
	)

	reqBody := geminiToolRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: system}}},
		Contents:          geminiConvertMessages(messages),
		Tools:             []geminiTool{{FunctionDeclarations: geminiConvertTools(tools)}},
		GenerationConfig:  geminiGenConfig{MaxOutputTokens: maxTokens},
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return Message{}, nil, fmt.Errorf("build gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Message{}, nil, fmt.Errorf("gemini HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result geminiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return Message{}, nil, fmt.Errorf("gemini response parse: %w", err)
	}
	if result.Error != nil {
		return Message{}, nil, fmt.Errorf("gemini error: %s", result.Error.Message)
	}
	if len(result.Candidates) == 0 {
		return Message{}, nil, fmt.Errorf("gemini returned empty response")
	}

	reply := Message{Role: "assistant"}
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Text != "" {
			reply.Text = part.Text
		}
		if part.FunctionCall != nil {
			reply.ToolUses = append(reply.ToolUses, ToolUse{
				// Gemini does not provide a unique call ID; use the name as a
				// stable substitute for round-tripping in functionResponse.
				ID:    part.FunctionCall.Name,
				Name:  part.FunctionCall.Name,
				Input: part.FunctionCall.Args,
			})
		}
	}

	var usage *Usage
	if result.UsageMetadata != nil {
		usage = &Usage{
			InputTokens:  result.UsageMetadata.PromptTokenCount,
			OutputTokens: result.UsageMetadata.CandidatesTokenCount,
		}
	}
	return reply, usage, nil
}

func geminiConvertMessages(messages []Message) []geminiContent {
	var out []geminiContent
	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		switch {
		case len(m.ToolUses) > 0:
			parts := make([]geminiPart, 0, len(m.ToolUses))
			for _, tu := range m.ToolUses {
				parts = append(parts, geminiPart{FunctionCall: &geminiFuncCall{Name: tu.Name, Args: tu.Input}})
			}
			out = append(out, geminiContent{Role: role, Parts: parts})
		case len(m.ToolResults) > 0:
			parts := make([]geminiPart, 0, len(m.ToolResults))
			for _, tr := range m.ToolResults {
				parts = append(parts, geminiPart{FunctionResponse: &geminiFuncResp{
					Name:     tr.ToolUseID,
					Response: map[string]any{"output": tr.Content},
				}})
			}
			out = append(out, geminiContent{Role: role, Parts: parts})
		default:
			out = append(out, geminiContent{Role: role, Parts: []geminiPart{{Text: m.Text}}})
		}
	}
	return out
}

func geminiConvertTools(tools []Tool) []geminiFuncDecl {
	decls := make([]geminiFuncDecl, 0, len(tools))
	for _, t := range tools {
		props := make(map[string]geminiToolProp, len(t.Parameters.Properties))
		for name, p := range t.Parameters.Properties {
			props[name] = geminiToolProp{
				Type:        geminiType(p.Type),
				Description: p.Description,
			}
		}
		decls = append(decls, geminiFuncDecl{
			Name:        t.Name,
			Description: t.Description,
			Parameters: geminiToolSchema{
				Type:       "OBJECT",
				Properties: props,
				Required:   t.Parameters.Required,
			},
		})
	}
	return decls
}

// geminiType maps JSON-schema type names to Gemini's uppercase equivalents.
func geminiType(t string) string {
	switch t {
	case "string":
		return "STRING"
	case "boolean":
		return "BOOLEAN"
	case "number", "integer":
		return "NUMBER"
	default:
		return "STRING"
	}
}
