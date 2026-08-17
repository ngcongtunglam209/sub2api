//go:build embed || unit

package web

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// maxHTMLCacheEntries bounds the cache regardless of what the key resolver
// hands us.
//
// The key is a resolved branding identity, so the live key count is already
// bounded by the number of configured reseller domains — operator-scale, tens.
// The cap is the second lock on the same door: it means no future caller can
// turn this map into an attacker-growable one by keying it on something
// client-controlled, and the failure mode if one tries is a cache miss rather
// than unbounded memory.
const maxHTMLCacheEntries = 64

// HTMLCache holds the rendered index.html per branding identity.
//
// Keyed rather than a single slot because the same deployment now renders
// different HTML for different hostnames: a reseller's site name, logo and
// subtitle are baked into the injected config, the <title> and the favicon.
// With one slot, whichever host rendered first would be served to every other
// host — reseller A's brand handed to reseller B's customers.
//
// The key is deliberately NOT the Host header. Host is attacker controlled, so
// keying on it would let anyone grow this map without bound by sending a
// million distinct hostnames. The caller resolves the host to a branding
// identity first (a reseller_domains row id, or "default" for everything that
// resolves to nothing) and keys second.
type HTMLCache struct {
	mu              sync.RWMutex
	entries         map[string]CachedHTML
	baseHTMLHash    string // Hash of the original index.html (immutable after build)
	settingsVersion uint64 // Incremented when settings change
}

// CachedHTML represents the cache state
type CachedHTML struct {
	Content []byte
	ETag    string
}

// NewHTMLCache creates a new HTML cache instance
func NewHTMLCache() *HTMLCache {
	return &HTMLCache{entries: make(map[string]CachedHTML)}
}

// SetBaseHTML initializes the cache with the base HTML template
func (c *HTMLCache) SetBaseHTML(baseHTML []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	hash := sha256.Sum256(baseHTML)
	c.baseHTMLHash = hex.EncodeToString(hash[:8]) // First 8 bytes for brevity
}

// Invalidate marks the cache as stale.
//
// Every identity at once: a settings change moves the values every rendering
// was derived from, and keeping a reseller's copy would leave them on the old
// global defaults for the fields they never overrode.
func (c *HTMLCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.settingsVersion++
	c.entries = make(map[string]CachedHTML)
}

// Get returns the cached HTML for one branding identity, or nil on a miss.
func (c *HTMLCache) Get(key string) *CachedHTML {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok || entry.Content == nil {
		return nil
	}
	return &CachedHTML{
		Content: entry.Content,
		ETag:    entry.ETag,
	}
}

// Set updates one branding identity's rendered HTML.
func (c *HTMLCache) Set(key string, html []byte, settingsJSON []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.entries == nil {
		c.entries = make(map[string]CachedHTML)
	}

	// At capacity with a new key, evict one entry rather than refusing to
	// cache: refusing would pin the miss on whichever identity arrived last and
	// leave it re-rendering on every request forever. Which entry goes is
	// unspecified — with the live key count bounded by the configured domains,
	// reaching this at all means something upstream is keying on the wrong
	// thing, and thrashing is the cheap symptom to have.
	if _, exists := c.entries[key]; !exists && len(c.entries) >= maxHTMLCacheEntries {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	c.entries[key] = CachedHTML{
		Content: html,
		ETag:    c.generateETag(key, settingsJSON),
	}
}

// Size reports how many branding identities are currently cached.
func (c *HTMLCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// generateETag creates an ETag from base HTML hash + branding identity + settings hash.
//
// The identity is in the hash, not just the settings: the ETag has to identify
// "this body", and two hosts rendering from the same settings snapshot are only
// the same body because they resolved to the same branding. Folding the key in
// means a validator can never be honoured across identities by anything that
// revalidates on the ETag alone.
func (c *HTMLCache) generateETag(key string, settingsJSON []byte) string {
	h := sha256.New()
	h.Write([]byte(key))
	h.Write([]byte{0})
	h.Write(settingsJSON)
	return `"` + c.baseHTMLHash + "-" + hex.EncodeToString(h.Sum(nil)[:8]) + `"`
}
