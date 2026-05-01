package ai

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
)

// sanitizeJSONResponse detects raw JSON responses from the AI model and
// formats them into human-readable Telegram HTML. Some models/proxies may
// output structured JSON (optionally wrapped in markdown code blocks)
// instead of natural language prose — this ensures Telegram users always
// see readable, formatted text.
//
// Handles both raw JSON and markdown-wrapped JSON:
//
//	```json\n{...}\n```
//	```\n{...}\n```
//	{...}
//
// If the text is not valid JSON, it is returned unchanged.
func sanitizeJSONResponse(text string) string {
	trimmed := strings.TrimSpace(text)

	// Strip markdown code block wrappers if present.
	// Proxies like marketriskmonitor.com may return Claude responses
	// wrapped in ```json ... ``` blocks.
	jsonStr := stripMarkdownCodeBlock(trimmed)

	if len(jsonStr) < 2 || jsonStr[0] != '{' {
		return text
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return text
	}

	var b strings.Builder
	renderJSONAsHTML(&b, data, 0)

	result := b.String()
	if result == "" {
		return text
	}
	return result
}

// stripMarkdownCodeBlock removes markdown code block fencing from text.
// Handles ```json, ```, and other language-tagged variants.
func stripMarkdownCodeBlock(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}

	// Find end of opening fence line
	firstNewline := strings.Index(s, "\n")
	if firstNewline < 0 {
		return s
	}

	// Check for closing fence
	if !strings.HasSuffix(s, "```") {
		return s
	}

	// Extract content between fences
	inner := s[firstNewline+1 : len(s)-3]
	return strings.TrimSpace(inner)
}

// renderJSONAsHTML recursively converts a JSON map into readable Telegram HTML.
// Simple values are rendered first, then arrays, then nested objects for
// a natural reading flow.
func renderJSONAsHTML(b *strings.Builder, data map[string]any, depth int) {
	keys := sortedKeysByType(data)
	indent := strings.Repeat("  ", depth)

	for _, key := range keys {
		label := snakeCaseToTitle(key)
		val := data[key]

		switch v := val.(type) {
		case string:
			b.WriteString(fmt.Sprintf("%s<b>%s:</b> %s\n", indent, label, html.EscapeString(v)))
		case float64:
			if v == float64(int64(v)) {
				b.WriteString(fmt.Sprintf("%s<b>%s:</b> %d\n", indent, label, int64(v)))
			} else {
				b.WriteString(fmt.Sprintf("%s<b>%s:</b> %.2f\n", indent, label, v))
			}
		case bool:
			b.WriteString(fmt.Sprintf("%s<b>%s:</b> %t\n", indent, label, v))
		case nil:
			// skip nil values
		case []any:
			b.WriteString(fmt.Sprintf("%s<b>%s:</b>\n", indent, label))
			for _, item := range v {
				switch iv := item.(type) {
				case string:
					b.WriteString(fmt.Sprintf("%s  • %s\n", indent, html.EscapeString(iv)))
				case map[string]any:
					renderJSONAsHTML(b, iv, depth+1)
					b.WriteString("\n")
				default:
					b.WriteString(fmt.Sprintf("%s  • %v\n", indent, item))
				}
			}
		case map[string]any:
			b.WriteString(fmt.Sprintf("\n%s<b>%s:</b>\n", indent, label))
			renderJSONAsHTML(b, v, depth+1)
		}
	}
}

// sortedKeysByType returns map keys sorted so that simple values come first,
// then arrays, then nested objects. Within each group, keys are alphabetical.
func sortedKeysByType(data map[string]any) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi, pj := jsonValuePriority(data[keys[i]]), jsonValuePriority(data[keys[j]])
		if pi != pj {
			return pi < pj
		}
		return keys[i] < keys[j]
	})
	return keys
}

// jsonValuePriority assigns a sort priority based on JSON value type.
func jsonValuePriority(v any) int {
	switch v.(type) {
	case string, float64, bool:
		return 0
	case nil:
		return 1
	case []any:
		return 2
	case map[string]any:
		return 3
	default:
		return 4
	}
}

// snakeCaseToTitle converts snake_case to Title Case.
// e.g. "regime_score" -> "Regime Score"
func snakeCaseToTitle(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
