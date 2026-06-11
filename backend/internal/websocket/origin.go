package websocket

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

// checkWSOrigin guards the WebSocket handshake against cross-site WebSocket
// hijacking: auth rides on cookies, so without this check any website could
// open an authenticated socket on behalf of a logged-in visitor.
//
// Allowed:
//   - Requests without an Origin header (non-browser clients; they cannot
//     carry a victim's cookies the way a browser does).
//   - Same-origin requests (the page and the WS endpoint share host:port —
//     covers the Vite dev proxy and the production Nginx proxy).
//   - Origins explicitly allow-listed in CORS_ALLOWED_ORIGINS (same variable
//     the HTTP CORS middleware uses).
func checkWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	return allowedWSOrigins()[strings.ToLower(origin)]
}

var (
	wsOriginsOnce sync.Once
	wsOrigins     map[string]bool
)

func allowedWSOrigins() map[string]bool {
	wsOriginsOnce.Do(func() {
		wsOrigins = make(map[string]bool)
		raw := os.Getenv("CORS_ALLOWED_ORIGINS")
		if strings.TrimSpace(raw) == "" {
			// Same development defaults as middleware.CORS.
			wsOrigins["http://localhost:5173"] = true
			wsOrigins["http://localhost:3000"] = true
			return
		}
		for _, o := range strings.Split(raw, ",") {
			if o = strings.ToLower(strings.TrimSpace(o)); o != "" {
				wsOrigins[o] = true
			}
		}
	})
	return wsOrigins
}
