package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFoldOpenAIFlatReasoningEffort(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantChanged bool
		wantEffort  string // "" 表示期望 reasoning.effort 不存在
	}{
		{
			name:        "flat snake case folds into reasoning.effort",
			body:        `{"model":"gpt-5.6-luna","reasoning_effort":"high"}`,
			wantChanged: true,
			wantEffort:  "high",
		},
		{
			name:        "flat camel case folds into reasoning.effort",
			body:        `{"model":"gpt-5.6-luna","reasoningEffort":"high"}`,
			wantChanged: true,
			wantEffort:  "high",
		},
		{
			name:        "non canonical value is canonicalized",
			body:        `{"reasoning_effort":"x-high"}`,
			wantChanged: true,
			wantEffort:  "xhigh",
		},
		{
			name:        "unknown value is passed through verbatim",
			body:        `{"reasoning_effort":"ludicrous"}`,
			wantChanged: true,
			wantEffort:  "ludicrous",
		},
		{
			name:        "existing reasoning.effort wins",
			body:        `{"reasoning":{"effort":"low"},"reasoning_effort":"max"}`,
			wantChanged: true,
			wantEffort:  "low",
		},
		{
			name:        "empty reasoning object is filled",
			body:        `{"reasoning":{"summary":"auto"},"reasoning_effort":"medium"}`,
			wantChanged: true,
			wantEffort:  "medium",
		},
		{
			name:        "non object reasoning is left untouched",
			body:        `{"reasoning":"none","reasoning_effort":"medium"}`,
			wantChanged: true,
			wantEffort:  "",
		},
		{
			name:        "non string flat value is dropped without folding",
			body:        `{"reasoning_effort":3}`,
			wantChanged: true,
			wantEffort:  "",
		},
		{
			name:        "body without flat key is untouched",
			body:        `{"reasoning":{"effort":"high"}}`,
			wantChanged: false,
			wantEffort:  "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, changed, err := foldOpenAIFlatReasoningEffort([]byte(tt.body))

			require.NoError(t, err)
			require.Equal(t, tt.wantChanged, changed)
			require.False(t, gjson.GetBytes(normalized, "reasoning_effort").Exists())
			require.False(t, gjson.GetBytes(normalized, "reasoningEffort").Exists())
			effort := gjson.GetBytes(normalized, "reasoning.effort")
			if tt.wantEffort == "" {
				require.False(t, effort.Exists())
				return
			}
			require.Equal(t, tt.wantEffort, effort.String())
		})
	}
}

func TestFoldOpenAIFlatReasoningEffortMap(t *testing.T) {
	t.Run("flat key folds into reasoning.effort", func(t *testing.T) {
		reqBody := map[string]any{"model": "gpt-5.6-luna", "reasoning_effort": "x-high"}

		require.True(t, foldOpenAIFlatReasoningEffortMap(reqBody))

		require.NotContains(t, reqBody, "reasoning_effort")
		reasoning, ok := reqBody["reasoning"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "xhigh", reasoning["effort"])
	})

	t.Run("existing reasoning.effort wins", func(t *testing.T) {
		reqBody := map[string]any{
			"reasoning":        map[string]any{"effort": "low"},
			"reasoningEffort":  "max",
			"reasoning_effort": "max",
		}

		require.True(t, foldOpenAIFlatReasoningEffortMap(reqBody))

		require.NotContains(t, reqBody, "reasoning_effort")
		require.NotContains(t, reqBody, "reasoningEffort")
		reasoning, ok := reqBody["reasoning"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "low", reasoning["effort"])
	})

	t.Run("body without flat key is untouched", func(t *testing.T) {
		reqBody := map[string]any{"model": "gpt-5.6-luna"}

		require.False(t, foldOpenAIFlatReasoningEffortMap(reqBody))
		require.NotContains(t, reqBody, "reasoning")
	})
}

// 回归：ChatGPT 内部 Responses 端点只认 reasoning.effort，客户端下发的扁平
// reasoning_effort 会被拒绝为 "Unsupported parameter: reasoning_effort"。
func TestNormalizeOpenAIPassthroughOAuthBodyFoldsFlatReasoningEffort(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-luna","reasoning_effort":"high","input":[]}`)

	normalized, changed, err := normalizeOpenAIPassthroughOAuthBody(body, false)

	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "reasoning_effort").Exists())
	require.Equal(t, "high", gjson.GetBytes(normalized, "reasoning.effort").String())
}

func TestApplyCodexOAuthTransformFoldsFlatReasoningEffort(t *testing.T) {
	reqBody := map[string]any{
		"model":            "gpt-5.6-luna",
		"reasoning_effort": "high",
		"input":            []any{map[string]any{"role": "user", "content": "hi"}},
	}

	result := applyCodexOAuthTransform(reqBody, true, false)

	require.True(t, result.Modified)
	require.NotContains(t, reqBody, "reasoning_effort")
	reasoning, ok := reqBody["reasoning"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "high", reasoning["effort"])
}

// 回归：上游只回 message（没有 error.code / error.param）时，重试仍要识别出
// 被拒绝的 reasoning_effort 并折叠进 reasoning.effort。
func TestNormalizeOpenAIResponsesRejectedFieldRetryBodyFoldsFlatReasoningEffort(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-luna","reasoning_effort":"high"}`)
	responseBody := []byte(`{"error":{"message":"Unsupported parameter: reasoning_effort","type":"invalid_request_error"}}`)

	retryBody, reason, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(http.StatusBadRequest, body, responseBody)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "reasoning_effort parameter rejection", reason)
	require.False(t, gjson.GetBytes(retryBody, "reasoning_effort").Exists())
	require.Equal(t, "high", gjson.GetBytes(retryBody, "reasoning.effort").String())
}

func TestNormalizeOpenAIResponsesRejectedFieldRetryBodySkipsAbsentFlatReasoningEffort(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-luna","reasoning":{"effort":"high"}}`)
	responseBody := []byte(`{"error":{"code":"unsupported_parameter","message":"Unsupported parameter: reasoning_effort","param":"reasoning_effort"}}`)

	retryBody, _, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(http.StatusBadRequest, body, responseBody)

	require.NoError(t, err)
	require.False(t, changed)
	require.Nil(t, retryBody)
}
