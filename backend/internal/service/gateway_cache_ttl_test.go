package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// collectEphemeralTTLs returns every ephemeral cache_control ttl in the body,
// in the tools → system → messages order the upstream validates.
func collectEphemeralTTLs(t *testing.T, body []byte) []string {
	t.Helper()

	ttls := make([]string, 0, 8)
	collect := func(container string) {
		gjson.GetBytes(body, container).ForEach(func(_, entry gjson.Result) bool {
			blocks := []gjson.Result{entry}
			if content := entry.Get("content"); content.IsArray() {
				blocks = content.Array()
			}
			for _, block := range blocks {
				cc := block.Get("cache_control")
				if cc.Exists() && cc.Get("type").String() == "ephemeral" {
					ttls = append(ttls, cc.Get("ttl").String())
				}
			}
			return true
		})
	}
	collect("tools")
	collect("system")
	collect("messages")
	return ttls
}

func TestResolveRequestCacheControlTTL(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "no cache_control falls back to the default",
			body: `{"model":"claude-fable-5","messages":[{"role":"user","content":"hi"}]}`,
			want: "5m",
		},
		{
			name: "client 1h on a message wins",
			body: `{"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`,
			want: "1h",
		},
		{
			name: "client 1h on system wins",
			body: `{"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"1h"}}],"messages":[]}`,
			want: "1h",
		},
		{
			name: "client 1h on tools wins",
			body: `{"tools":[{"name":"a","input_schema":{},"cache_control":{"type":"ephemeral","ttl":"1h"}}]}`,
			want: "1h",
		},
		{
			name: "top level 1h wins",
			body: `{"cache_control":{"type":"ephemeral","ttl":"1h"},"messages":[]}`,
			want: "1h",
		},
		{
			name: "client 5m stays 5m",
			body: `{"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"5m"}}]}`,
			want: "5m",
		},
		{
			name: "non ephemeral 1h is ignored",
			body: `{"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"persistent","ttl":"1h"}}]}]}`,
			want: "5m",
		},
		{
			name: "empty body falls back to the default",
			body: ``,
			want: "5m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, resolveRequestCacheControlTTL([]byte(tt.body)))
		})
	}
}

// 回归：客户端（Claude Code）用 ttl=1h，代理再往 tools[-1] 打一个 5m 断点时，
// 上游按 tools → system → messages 判定 ttl 递增，整个请求 400：
// "a ttl='1h' cache_control block must not come after a ttl='5m' cache_control block"。
func TestApplyToolsLastCacheBreakpointFollowsClientTTL(t *testing.T) {
	body := []byte(`{"tools":[{"name":"Read","input_schema":{}}],"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`)

	out := applyToolsLastCacheBreakpoint(body)

	require.Equal(t, "1h", gjson.GetBytes(out, "tools.0.cache_control.ttl").String())
	require.Equal(t, []string{"1h", "1h"}, collectEphemeralTTLs(t, out))
}

func TestApplyToolsLastCacheBreakpointKeepsDefaultWithoutClientTTL(t *testing.T) {
	body := []byte(`{"tools":[{"name":"Read","input_schema":{}}],"messages":[{"role":"user","content":"hi"}]}`)

	out := applyToolsLastCacheBreakpoint(body)

	require.Equal(t, "5m", gjson.GetBytes(out, "tools.0.cache_control.ttl").String())
}

func TestAddMessageCacheBreakpointsFollowsClientTTL(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"1h"}}],"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)

	out := addMessageCacheBreakpoints(body)

	require.Equal(t, "1h", gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
	require.Equal(t, []string{"1h", "1h"}, collectEphemeralTTLs(t, out))
}

func TestAddMessageCacheBreakpointsUpgradesStringContentWithClientTTL(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"1h"}}],"messages":[{"role":"user","content":"hi"}]}`)

	out := addMessageCacheBreakpoints(body)

	require.Equal(t, "hi", gjson.GetBytes(out, "messages.0.content.0.text").String())
	require.Equal(t, "1h", gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
}

// 回归：system prompt 注入阶段落 5m、客户端 messages 自带 1h 的混合请求体，
// 统一化之后必须只剩一种 ttl。
func TestNormalizeCacheControlTTLUniformCollapsesMixedTTLs(t *testing.T) {
	body := []byte(`{"tools":[{"name":"Read","input_schema":{},"cache_control":{"type":"ephemeral","ttl":"5m"}}],"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"5m"}}],"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`)

	out := normalizeCacheControlTTLUniform(body)

	require.Equal(t, []string{"1h", "1h", "1h"}, collectEphemeralTTLs(t, out))
}

func TestNormalizeCacheControlTTLUniformLeavesAllDefaultBodyAlone(t *testing.T) {
	body := []byte(`{"tools":[{"name":"Read","input_schema":{},"cache_control":{"type":"ephemeral","ttl":"5m"}}],"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","ttl":"5m"}}]}]}`)

	out := normalizeCacheControlTTLUniform(body)

	require.JSONEq(t, string(body), string(out))
}

func TestNormalizeCacheControlTTLUniformSkipsNonEphemeralBlocks(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"sys","cache_control":{"type":"persistent","ttl":"5m"}}],"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`)

	out := normalizeCacheControlTTLUniform(body)

	require.Equal(t, "5m", gjson.GetBytes(out, "system.0.cache_control.ttl").String())
	require.Equal(t, "1h", gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
}

// 回归：完整还原客户上报的请求形状 —— Claude Code 客户端 1h + 代理注入的 system
// 与 tools 断点，走完 messages/tools 两个断点阶段再统一化后不得再有混合 ttl。
func TestProxyBreakpointsStayUniformForClaudeCodeOneHourClient(t *testing.T) {
	body := []byte(`{"model":"claude-fable-5","system":[{"type":"text","text":"You are Claude Code","cache_control":{"type":"ephemeral","ttl":"5m"}}],"tools":[{"name":"Read","input_schema":{}}],"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`)

	out := normalizeCacheControlTTLUniform(applyToolsLastCacheBreakpoint(body))

	ttls := collectEphemeralTTLs(t, out)
	require.NotEmpty(t, ttls)
	for _, ttl := range ttls {
		require.Equal(t, ttls[0], ttl, "mixed ttls remain: %v", ttls)
	}
}
