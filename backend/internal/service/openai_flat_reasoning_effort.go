package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// openAIFlatReasoningEffortFields lists the Chat Completions style effort keys
// that clients keep sending on Responses requests. The Responses schema only
// knows reasoning.effort, so ChatGPT internal and OpenAI upstreams reject the
// flat keys with "Unsupported parameter: reasoning_effort".
var openAIFlatReasoningEffortFields = []string{"reasoning_effort", "reasoningEffort"}

// canonicalOpenAIFlatReasoningEffort canonicalizes a flat effort value. Unknown
// values are passed through verbatim so upstream stays the source of truth.
func canonicalOpenAIFlatReasoningEffort(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if canonical := NormalizeMaxReasoningEffort(value); canonical != "" {
		return canonical
	}
	return value
}

// foldOpenAIFlatReasoningEffort moves a flat reasoning_effort / reasoningEffort
// field into the Responses-native reasoning.effort object and drops the flat
// key. An existing reasoning.effort always wins, and a non-object reasoning
// value is left untouched so nothing is silently overwritten.
func foldOpenAIFlatReasoningEffort(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	normalized := body
	changed := false
	for _, field := range openAIFlatReasoningEffortFields {
		value := gjson.GetBytes(normalized, field)
		if !value.Exists() {
			continue
		}

		if effort := canonicalOpenAIFlatReasoningEffort(value.String()); effort != "" && value.Type == gjson.String {
			reasoning := gjson.GetBytes(normalized, "reasoning")
			foldable := !reasoning.Exists() ||
				(reasoning.IsObject() && strings.TrimSpace(reasoning.Get("effort").String()) == "")
			if foldable {
				next, err := sjson.SetBytes(normalized, "reasoning.effort", effort)
				if err != nil {
					return body, false, fmt.Errorf("fold flat %s into reasoning.effort: %w", field, err)
				}
				normalized = next
			}
		}

		next, err := sjson.DeleteBytes(normalized, field)
		if err != nil {
			return body, false, fmt.Errorf("delete flat %s: %w", field, err)
		}
		normalized = next
		changed = true
	}
	return normalized, changed, nil
}

// foldOpenAIFlatReasoningEffortMap is the decoded-body counterpart of
// foldOpenAIFlatReasoningEffort and reports whether the body changed.
func foldOpenAIFlatReasoningEffortMap(reqBody map[string]any) bool {
	if len(reqBody) == 0 {
		return false
	}

	changed := false
	for _, field := range openAIFlatReasoningEffortFields {
		raw, ok := reqBody[field]
		if !ok {
			continue
		}

		if text, isString := raw.(string); isString {
			if effort := canonicalOpenAIFlatReasoningEffort(text); effort != "" {
				switch reasoning := reqBody["reasoning"].(type) {
				case map[string]any:
					if current, _ := reasoning["effort"].(string); strings.TrimSpace(current) == "" {
						reasoning["effort"] = effort
					}
				case nil:
					if _, exists := reqBody["reasoning"]; !exists {
						reqBody["reasoning"] = map[string]any{"effort": effort}
					}
				}
			}
		}

		delete(reqBody, field)
		changed = true
	}
	return changed
}
