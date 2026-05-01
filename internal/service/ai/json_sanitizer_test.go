package ai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeJSONResponse_NonJSON(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"plain text", "This is a normal response."},
		{"HTML text", "<b>Bold</b> and <i>italic</i>"},
		{"empty", ""},
		{"starts with bracket", "[not a valid object]"},
		{"invalid JSON", "{not json}"},
		{"partial JSON", `{"key": "value`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeJSONResponse(tc.input)
			assert.Equal(t, tc.input, result, "non-JSON input should be returned unchanged")
		})
	}
}

func TestSanitizeJSONResponse_SimpleJSON(t *testing.T) {
	input := `{"state": "Fragile", "score": 76, "active": true}`
	result := sanitizeJSONResponse(input)

	assert.NotEqual(t, input, result, "JSON should be transformed")
	assert.Contains(t, result, "<b>State:</b> Fragile")
	assert.Contains(t, result, "<b>Score:</b> 76")
	assert.Contains(t, result, "<b>Active:</b> true")
}

func TestSanitizeJSONResponse_NestedJSON(t *testing.T) {
	input := `{
		"state": "Fragile / Hedged",
		"regime_score": 76,
		"overrides_triggered": [
			"Equity breadth deteriorating",
			"Small-cap underperformance"
		],
		"metrics": {
			"spx_200dma": {
				"value": "2.1% above 200DMA",
				"score": 72,
				"status": "neutral"
			}
		},
		"summary": {
			"one_line": "Market structure shows cracks"
		}
	}`

	result := sanitizeJSONResponse(input)

	assert.Contains(t, result, "<b>State:</b> Fragile / Hedged")
	assert.Contains(t, result, "<b>Regime Score:</b> 76")
	assert.Contains(t, result, "Equity breadth deteriorating")
	assert.Contains(t, result, "Small-cap underperformance")
	assert.Contains(t, result, "<b>Metrics:</b>")
	assert.Contains(t, result, "<b>Value:</b> 2.1% above 200DMA")
	assert.Contains(t, result, "<b>Summary:</b>")
	assert.Contains(t, result, "Market structure shows cracks")
}

func TestSanitizeJSONResponse_HTMLEscaping(t *testing.T) {
	input := `{"note": "SPX > 5000 & VIX < 20"}`
	result := sanitizeJSONResponse(input)

	assert.Contains(t, result, "&amp;", "ampersand should be escaped")
	assert.Contains(t, result, "&gt;", "greater-than should be escaped")
	assert.Contains(t, result, "&lt;", "less-than should be escaped")
}

func TestSanitizeJSONResponse_SortOrder(t *testing.T) {
	input := `{
		"metrics": {"a": 1},
		"items": ["x"],
		"name": "test",
		"score": 50
	}`

	result := sanitizeJSONResponse(input)

	nameIdx := strings.Index(result, "Name:")
	scoreIdx := strings.Index(result, "Score:")
	itemsIdx := strings.Index(result, "Items:")
	metricsIdx := strings.Index(result, "Metrics:")

	assert.True(t, nameIdx < itemsIdx, "simple values should come before arrays")
	assert.True(t, scoreIdx < itemsIdx, "simple values should come before arrays")
	assert.True(t, itemsIdx < metricsIdx, "arrays should come before nested objects")
}

func TestSnakeCaseToTitle(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"regime_score", "Regime Score"},
		{"state", "State"},
		{"spx_200dma", "Spx 200dma"},
		{"one_line", "One Line"},
		{"composite_risk_score", "Composite Risk Score"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, snakeCaseToTitle(tc.input))
		})
	}
}

func TestSanitizeJSONResponse_FloatValues(t *testing.T) {
	input := `{"integer_val": 76, "float_val": 3.14}`
	result := sanitizeJSONResponse(input)

	assert.Contains(t, result, "<b>Float Val:</b> 3.14")
	assert.Contains(t, result, "<b>Integer Val:</b> 76")
	assert.NotContains(t, result, "76.00", "integers should not show decimal places")
}

func TestSanitizeJSONResponse_WhitespaceHandling(t *testing.T) {
	input := `   {"key": "value"}   `
	result := sanitizeJSONResponse(input)
	assert.Contains(t, result, "<b>Key:</b> value")
}

func TestSanitizeJSONResponse_MarkdownCodeBlock(t *testing.T) {
	// This is the exact format returned by the marketriskmonitor.com proxy:
	// Claude's text response is wrapped in ```json ... ```
	input := "```json\n{\"state\": \"Fragile / Hedged\", \"regime_score\": 76}\n```"
	result := sanitizeJSONResponse(input)

	assert.NotEqual(t, input, result, "markdown-wrapped JSON should be transformed")
	assert.Contains(t, result, "<b>State:</b> Fragile / Hedged")
	assert.Contains(t, result, "<b>Regime Score:</b> 76")
	assert.NotContains(t, result, "```", "markdown fencing should be stripped")
}

func TestSanitizeJSONResponse_MarkdownCodeBlockNoLang(t *testing.T) {
	input := "```\n{\"key\": \"value\"}\n```"
	result := sanitizeJSONResponse(input)

	assert.Contains(t, result, "<b>Key:</b> value")
	assert.NotContains(t, result, "```")
}

func TestSanitizeJSONResponse_MarkdownCodeBlockNonJSON(t *testing.T) {
	// Code block with non-JSON content should be returned unchanged
	input := "```python\nprint('hello')\n```"
	result := sanitizeJSONResponse(input)
	assert.Equal(t, input, result, "non-JSON code blocks should be returned unchanged")
}

func TestStripMarkdownCodeBlock(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"json fence", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"plain fence", "```\n{\"a\":1}\n```", `{"a":1}`},
		{"no fence", `{"a":1}`, `{"a":1}`},
		{"no closing fence", "```json\n{\"a\":1}", "```json\n{\"a\":1}"},
		{"no newline", "```json```", "```json```"},
		{"plain text", "hello world", "hello world"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, stripMarkdownCodeBlock(tc.input))
		})
	}
}

func TestSanitizeJSONResponse_FullProxyResponse(t *testing.T) {
	// Simulate the exact response from the marketriskmonitor.com proxy
	input := "```json\n{\n  \"regime_score\": 76,\n  \"fragility_score\": 51,\n  \"state\": \"Fragile / Hedged\",\n  \"action_stance\": \"Reduce exposure, raise cash\",\n  \"overrides_triggered\": [\n    \"Equity breadth deteriorating\"\n  ],\n  \"summary\": {\n    \"one_line\": \"Market shows cracks\"\n  }\n}\n```"

	result := sanitizeJSONResponse(input)

	assert.Contains(t, result, "<b>Regime Score:</b> 76")
	assert.Contains(t, result, "<b>Fragility Score:</b> 51")
	assert.Contains(t, result, "<b>State:</b> Fragile / Hedged")
	assert.Contains(t, result, "<b>Action Stance:</b> Reduce exposure, raise cash")
	assert.Contains(t, result, "Equity breadth deteriorating")
	assert.Contains(t, result, "<b>Summary:</b>")
	assert.Contains(t, result, "Market shows cracks")
	assert.NotContains(t, result, "```")
}
