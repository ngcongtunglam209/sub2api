//go:build embed || unit

package web

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// HTMLCache holds the single rendered index.html.
//
// One slot, because one deployment serves one site: the injected config, the
// <title> and the favicon all come from the global site settings, so every
// request renders the same body. The only thing that moves it is a settings
// change, and that invalidates the slot outright.
type HTMLCache struct {
	mu              sync.RWMutex
	entry           CachedHTML
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
	return &HTMLCache{}
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
// A settings change moves the values the rendering was derived from, so the
// stored body is dropped rather than patched.
func (c *HTMLCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.settingsVersion++
	c.entry = CachedHTML{}
}

// Get returns the cached HTML, or nil on a miss.
func (c *HTMLCache) Get() *CachedHTML {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.entry.Content == nil {
		return nil
	}
	return &CachedHTML{
		Content: c.entry.Content,
		ETag:    c.entry.ETag,
	}
}

// Set stores the rendered HTML.
func (c *HTMLCache) Set(html []byte, settingsJSON []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entry = CachedHTML{
		Content: html,
		ETag:    c.generateETag(settingsJSON),
	}
}

// generateETag creates an ETag from base HTML hash + settings hash.
func (c *HTMLCache) generateETag(settingsJSON []byte) string {
	h := sha256.New()
	h.Write(settingsJSON)
	return `"` + c.baseHTMLHash + "-" + hex.EncodeToString(h.Sum(nil)[:8]) + `"`
}
