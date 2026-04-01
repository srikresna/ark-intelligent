package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// ErrAIRateLimited is returned when the AI rate limiter rejects a request.
var ErrAIRateLimited = errors.New("AI rate limited — try again later")

var geminiLog = logger.Component("gemini")

// GeminiClient wraps the Google Generative AI SDK for structured
// financial analysis prompts. It manages the client lifecycle,
// model configuration, and retry logic.
type GeminiClient struct {
	client    *genai.Client
	model     *genai.GenerativeModel
	apiKey    string
	modelName string
	limiter   *aiRateLimiter
}

// NewGeminiClient creates a Gemini client with the given API key, model name, and rate limits.
func NewGeminiClient(ctx context.Context, apiKey string, modelName string, maxRPM int, maxDaily int) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}

	model := client.GenerativeModel(modelName)

	// Configure for financial analysis
	model.SetTemperature(0.3) // low creativity, high precision
	model.SetTopP(0.8)
	model.SetTopK(40)
	model.SetMaxOutputTokens(4096)

	// Safety settings: allow financial discussion
	model.SafetySettings = []*genai.SafetySetting{
		{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockOnlyHigh},
		{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockOnlyHigh},
	}

	return &GeminiClient{
		client:    client,
		model:     model,
		apiKey:    apiKey,
		modelName: modelName,
		limiter:   newAIRateLimiter(maxRPM, maxDaily),
	}, nil
}

// Generate sends a prompt and returns the text response.
// Includes retry logic with exponential backoff for transient errors.
func (gc *GeminiClient) Generate(ctx context.Context, prompt string) (string, error) {
	if gc.limiter != nil && !gc.limiter.Allow() {
		rpm, used, max := gc.limiter.Stats()
		geminiLog.Warn().Int("rpm", rpm).Int("daily_used", used).Int("daily_max", max).Msg("AI rate limited")
		return "", ErrAIRateLimited
	}

	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			geminiLog.Warn().Int("attempt", attempt).Dur("backoff", backoff).Msg("retrying request")
			time.Sleep(backoff)
		}

		resp, err := gc.model.GenerateContent(ctx, genai.Text(prompt))
		if err != nil {
			lastErr = fmt.Errorf("generate (attempt %d): %w", attempt+1, err)
			// Retry on transient errors
			if isTransient(err) {
				continue
			}
			return "", lastErr
		}

		text := extractText(resp)
		if text == "" {
			lastErr = fmt.Errorf("empty response (attempt %d)", attempt+1)
			continue
		}

		return text, nil
	}

	return "", fmt.Errorf("all retries exhausted: %w", lastErr)
}

// GenerateWithSystem sends a prompt with a system instruction.
func (gc *GeminiClient) GenerateWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if gc.limiter != nil && !gc.limiter.Allow() {
		rpm, used, max := gc.limiter.Stats()
		geminiLog.Warn().Int("rpm", rpm).Int("daily_used", used).Int("daily_max", max).Msg("AI rate limited")
		return "", ErrAIRateLimited
	}

	// Clone model config with system instruction
	model := gc.client.GenerativeModel(gc.modelName)
	model.SetTemperature(0.3)
	model.SetTopP(0.8)
	model.SetTopK(40)
	model.SetMaxOutputTokens(4096)
	model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * time.Second)
		}

		resp, err := model.GenerateContent(ctx, genai.Text(userPrompt))
		if err != nil {
			lastErr = err
			if isTransient(err) {
				continue
			}
			return "", fmt.Errorf("generate with system: %w", err)
		}

		text := extractText(resp)
		if text != "" {
			return text, nil
		}
		lastErr = fmt.Errorf("empty response")
	}

	return "", fmt.Errorf("all retries exhausted: %w", lastErr)
}

// Close releases the Gemini client resources.
func (gc *GeminiClient) Close() {
	if gc.client != nil {
		gc.client.Close()
	}
}

// --- helpers ---

// extractText pulls the text content from a Gemini response.
func extractText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 {
		return ""
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return ""
	}

	var parts []string
	for _, part := range candidate.Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			parts = append(parts, string(textPart))
		}
	}

	return strings.Join(parts, "")
}

// isTransient checks if an error is likely transient (worth retrying).
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "deadline") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "unavailable")
}
