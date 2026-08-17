//go:build unit

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/branding"
)

const brandingTestBaseHTML = `<!doctype html><html><head>` +
	`<link rel="icon" type="image/svg+xml" href="/favicon.svg" />` +
	`<title>Sub2API - AI API Gateway</title>` +
	`</head><body><div id="app"></div></body></html>`

// settingsFor renders the settings snapshot GetPublicSettings would produce for
// one host: the deployment's own values with the host's overrides applied.
func settingsFor(t *testing.T, host branding.Host) []byte {
	t.Helper()

	cfg := map[string]any{
		"site_name":     "House Brand",
		"site_logo":     "https://cdn.house.example/logo.png",
		"site_subtitle": "House Gateway",
	}
	if host.SiteName != "" {
		cfg["site_name"] = host.SiteName
	}
	if host.SiteLogo != "" {
		cfg["site_logo"] = host.SiteLogo
	}
	if host.SiteSubtitle != "" {
		cfg["site_subtitle"] = host.SiteSubtitle
	}

	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	return raw
}

// serve mimics FrontendServer.serveIndexHTML's cache interaction: resolve the
// branding identity, look it up, render and store on a miss.
func serve(t *testing.T, cache *HTMLCache, host branding.Host) CachedHTML {
	t.Helper()

	key := branding.FromContext(branding.NewContext(context.Background(), host)).CacheKey()
	if cached := cache.Get(key); cached != nil {
		return *cached
	}

	settingsJSON := settingsFor(t, host)
	cache.Set(key, renderIndexHTML([]byte(brandingTestBaseHTML), settingsJSON), settingsJSON)

	cached := cache.Get(key)
	require.NotNil(t, cached)
	return *cached
}

// The leak the keyed cache exists to prevent: with one global slot, whichever
// reseller rendered first was served to every other reseller's customers.
func TestTwoResellerHostsGetDifferentHTMLAndETag(t *testing.T) {
	cache := NewHTMLCache()
	cache.SetBaseHTML([]byte(brandingTestBaseHTML))

	alpha := serve(t, cache, branding.Host{DomainID: 7, SiteName: "Alpha", SiteLogo: "https://cdn.alpha.example/logo.png"})
	beta := serve(t, cache, branding.Host{DomainID: 9, SiteName: "Beta", SiteLogo: "https://cdn.beta.example/logo.png"})

	require.NotEqual(t, string(alpha.Content), string(beta.Content))
	require.NotEqual(t, alpha.ETag, beta.ETag)

	require.Contains(t, string(alpha.Content), "<title>Alpha - AI API Gateway</title>")
	require.NotContains(t, string(alpha.Content), "Beta")
	require.Contains(t, string(alpha.Content), `<link rel="icon" href="https://cdn.alpha.example/logo.png" />`)

	require.Contains(t, string(beta.Content), "<title>Beta - AI API Gateway</title>")
	require.NotContains(t, string(beta.Content), "Alpha")
	require.Contains(t, string(beta.Content), `<link rel="icon" href="https://cdn.beta.example/logo.png" />`)

	// A second request for each host is served from that host's own entry.
	require.Equal(t, string(alpha.Content), string(serve(t, cache, branding.Host{DomainID: 7, SiteName: "Alpha", SiteLogo: "https://cdn.alpha.example/logo.png"}).Content))
	require.Equal(t, 2, cache.Size())
}

// The canonical host, an unknown host and a domain with nothing configured all
// render today's branding, from one shared entry.
func TestUnbrandedHostsShareTheDefaultEntry(t *testing.T) {
	cache := NewHTMLCache()
	cache.SetBaseHTML([]byte(brandingTestBaseHTML))

	canonical := serve(t, cache, branding.Host{})                // canonical or unknown host
	unconfigured := serve(t, cache, branding.Host{DomainID: 11}) // registered, no override

	require.Equal(t, string(canonical.Content), string(unconfigured.Content))
	require.Equal(t, canonical.ETag, unconfigured.ETag)
	require.Contains(t, string(canonical.Content), "<title>House Brand - AI API Gateway</title>")
	require.Contains(t, string(canonical.Content), `<link rel="icon" href="https://cdn.house.example/logo.png" />`)
	require.Equal(t, 1, cache.Size())

	// And a reseller's entry never displaces it.
	serve(t, cache, branding.Host{DomainID: 7, SiteName: "Alpha"})
	require.Equal(t, string(canonical.Content), string(serve(t, cache, branding.Host{}).Content))
}

// Host is attacker-controlled. Keying on it would let anyone grow this map
// without bound; keying on the resolved identity means a flood of unknown
// hostnames all land on the one default entry.
func TestCacheDoesNotGrowWithUnresolvedHosts(t *testing.T) {
	cache := NewHTMLCache()
	cache.SetBaseHTML([]byte(brandingTestBaseHTML))

	for i := 0; i < 5000; i++ {
		// Every one of these resolved to nothing — an unknown host, which is
		// the shape a prober generates.
		serve(t, cache, branding.Host{})
	}

	require.Equal(t, 1, cache.Size())
}

// Second lock on the same door: even if a future caller keys this on something
// client-controlled, the map stays bounded.
func TestCacheIsBoundedRegardlessOfKeyCount(t *testing.T) {
	cache := NewHTMLCache()
	cache.SetBaseHTML([]byte(brandingTestBaseHTML))

	for i := 0; i < maxHTMLCacheEntries*10; i++ {
		settingsJSON := []byte(fmt.Sprintf(`{"site_name":"brand-%d"}`, i))
		cache.Set(fmt.Sprintf("domain:%d", i), []byte(fmt.Sprintf("<html>%d</html>", i)), settingsJSON)
		require.LessOrEqual(t, cache.Size(), maxHTMLCacheEntries)
	}
}

// A settings change moves the values every rendering was derived from,
// including the fields a reseller never overrode.
func TestInvalidateClearsEveryIdentity(t *testing.T) {
	cache := NewHTMLCache()
	cache.SetBaseHTML([]byte(brandingTestBaseHTML))

	serve(t, cache, branding.Host{})
	serve(t, cache, branding.Host{DomainID: 7, SiteName: "Alpha"})
	require.Equal(t, 2, cache.Size())

	cache.Invalidate()

	require.Zero(t, cache.Size())
	require.Nil(t, cache.Get(branding.DefaultCacheKey))
	require.Nil(t, cache.Get("domain:7"))
}

// A reseller supplies these strings, so they are lower-privilege input than the
// global settings an admin types. Both places they land have to hold: the tab
// title goes through HTML escaping, and the injected config goes through
// encoding/json, which escapes `<` — without that a site name containing
// `</script>` would close the config script and run as markup.
func TestRenderedBrandingCannotBreakOutOfThePage(t *testing.T) {
	settingsJSON := settingsFor(t, branding.Host{
		DomainID: 7,
		SiteName: "</title></script><script>alert(1)</script>",
	})
	rendered := string(renderIndexHTML([]byte(brandingTestBaseHTML), settingsJSON))

	require.NotContains(t, rendered, "<script>alert(1)</script>")
	require.NotContains(t, rendered, "</title></script>")
	require.Contains(t, rendered, "&lt;script&gt;", "the tab title is HTML-escaped")
	require.Contains(t, rendered, "\\u003cscript\\u003e", "the injected config is JSON-escaped")

	// Exactly the one <title> and the one config <script> the page started with.
	require.Equal(t, 1, strings.Count(rendered, "</title>"))
	require.Equal(t, 1, strings.Count(rendered, "</script>"))
}
