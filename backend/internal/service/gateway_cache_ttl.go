package service

import (
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/tidwall/gjson"
)

// forEachCacheControlBlock visits every cache_control object in an Anthropic
// request body: the top-level one, then system, messages content, and tools.
// Returning false from visit stops the walk.
func forEachCacheControlBlock(body []byte, visit func(path string, cacheControl gjson.Result) bool) {
	if len(body) == 0 || visit == nil {
		return
	}

	if cc := gjson.GetBytes(body, "cache_control"); cc.Exists() {
		if !visit("cache_control", cc) {
			return
		}
	}

	walking := true
	visitBlock := func(path string, block gjson.Result) bool {
		cc := block.Get("cache_control")
		if !cc.Exists() {
			return true
		}
		walking = visit(path+".cache_control", cc)
		return walking
	}

	if system := gjson.GetBytes(body, "system"); system.IsArray() {
		idx := -1
		system.ForEach(func(_, block gjson.Result) bool {
			idx++
			return visitBlock(fmt.Sprintf("system.%d", idx), block)
		})
		if !walking {
			return
		}
	}

	if messages := gjson.GetBytes(body, "messages"); messages.IsArray() {
		msgIdx := -1
		messages.ForEach(func(_, msg gjson.Result) bool {
			msgIdx++
			content := msg.Get("content")
			if !content.IsArray() {
				return true
			}
			blockIdx := -1
			content.ForEach(func(_, block gjson.Result) bool {
				blockIdx++
				return visitBlock(fmt.Sprintf("messages.%d.content.%d", msgIdx, blockIdx), block)
			})
			return walking
		})
		if !walking {
			return
		}
	}

	if tools := gjson.GetBytes(body, "tools"); tools.IsArray() {
		idx := -1
		tools.ForEach(func(_, tool gjson.Result) bool {
			idx++
			return visitBlock(fmt.Sprintf("tools.%d", idx), tool)
		})
	}
}

// resolveRequestCacheControlTTL reports the ttl the proxy must stamp on every
// cache_control breakpoint it injects into one request.
//
// Anthropic processes cache_control blocks in tools → system → messages order
// and rejects a ttl="1h" block that follows a ttl="5m" one. Deciding the ttl per
// block therefore cannot be correct on its own: a Claude Code client sends 1h on
// its own blocks, and a proxy breakpoint stamped with the 5m default lands in
// tools or system — ahead of the client's 1h — which is exactly that 400.
//
// So the ttl is decided once for the whole request: honour the longest ttl the
// client already asked for, and fall back to the 5m default when it asked for
// nothing. That keeps the "do not spend 1h cache quota unless the client wants
// it" policy while making a mixed body impossible.
func resolveRequestCacheControlTTL(body []byte) string {
	ttl := claude.DefaultCacheControlTTL
	forEachCacheControlBlock(body, func(_ string, cacheControl gjson.Result) bool {
		if cacheControl.Get("type").String() != "ephemeral" {
			return true
		}
		if cacheControl.Get("ttl").String() == cacheTTLTarget1h {
			ttl = cacheTTLTarget1h
			return false
		}
		return true
	})
	return ttl
}

// normalizeCacheControlTTLUniform collapses every ephemeral cache_control block
// onto the request's resolved ttl. A uniform body can never violate the
// tools → system → messages ordering rule, whichever stage stamped which block.
//
// A resolved ttl of 5m means no block asked for 1h, so nothing can be out of
// order and the body is returned untouched — an omitted ttl already means 5m,
// and writing it out would rewrite a passthrough client's bytes for nothing.
func normalizeCacheControlTTLUniform(body []byte) []byte {
	ttl := resolveRequestCacheControlTTL(body)
	if ttl == claude.DefaultCacheControlTTL {
		return body
	}
	return forceEphemeralCacheControlTTL(body, ttl)
}
