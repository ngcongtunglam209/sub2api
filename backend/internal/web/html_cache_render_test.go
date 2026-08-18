//go:build unit

package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const renderTestBaseHTML = `<!doctype html><html><head>` +
	`<link rel="icon" type="image/svg+xml" href="/favicon.svg" />` +
	`<title>Sub2API - AI API Gateway</title>` +
	`</head><body><div id="app"></div></body></html>`

// settingsJSONFor renders the snapshot GetPublicSettings would produce: the
// deployment's own site values, which is all there is now that every host
// renders the same page.
func settingsJSONFor(t *testing.T, siteName string) []byte {
	t.Helper()

	raw, err := json.Marshal(map[string]any{
		"site_name":     siteName,
		"site_logo":     "https://cdn.house.example/logo.png",
		"site_subtitle": "House Gateway",
	})
	require.NoError(t, err)
	return raw
}

// site_name is admin-typed, but it still reaches the page through two sinks and
// both have to hold: the tab title goes through HTML escaping, and the injected
// config goes through encoding/json, which escapes `<` — without that a site
// name containing `</script>` would close the config script and run as markup.
func TestRenderedSettingsCannotBreakOutOfThePage(t *testing.T) {
	settingsJSON := settingsJSONFor(t, "</title></script><script>alert(1)</script>")
	rendered := string(renderIndexHTML([]byte(renderTestBaseHTML), settingsJSON))

	require.NotContains(t, rendered, "<script>alert(1)</script>")
	require.NotContains(t, rendered, "</title></script>")
	require.Contains(t, rendered, "&lt;script&gt;", "the tab title is HTML-escaped")
	require.Contains(t, rendered, "\\u003cscript\\u003e", "the injected config is JSON-escaped")

	// Exactly the one <title> and the one config <script> the page started with.
	require.Equal(t, 1, strings.Count(rendered, "</title>"))
	require.Equal(t, 1, strings.Count(rendered, "</script>"))
}

// The rendered page is what the cache stores, so a render still has to produce
// the site's own name and logo — the path reseller overrides used to ride on.
func TestRenderedPageCarriesTheGlobalSiteIdentity(t *testing.T) {
	settingsJSON := settingsJSONFor(t, "House Brand")
	rendered := string(renderIndexHTML([]byte(renderTestBaseHTML), settingsJSON))

	require.Contains(t, rendered, "House Brand")
	require.Contains(t, rendered, "https://cdn.house.example/logo.png")
}
