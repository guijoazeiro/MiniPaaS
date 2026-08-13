package ws

import (
	"net/http"
	"strings"
)

// websocketOriginChecker accepts non-browser clients (which do not send an
// Origin header) while requiring browser clients to come from the configured
// dashboard origin. This keeps CLI WebSocket access working without allowing
// arbitrary pages to reuse a dashboard session cookie.
func websocketOriginChecker(origins ...string) func(*http.Request) bool {
	allowed := ""
	if len(origins) > 0 {
		allowed = strings.TrimRight(strings.TrimSpace(origins[0]), "/")
	}
	return func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		return allowed != "" && origin == allowed
	}
}
