package ws

import (
	"net/http/httptest"
	"testing"
)

func TestWebsocketOriginChecker(t *testing.T) {
	checker := websocketOriginChecker("http://localhost:3000/")
	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "cli without origin", want: true},
		{name: "configured dashboard", origin: "http://localhost:3000", want: true},
		{name: "different port", origin: "http://localhost:3001", want: false},
		{name: "different host", origin: "https://evil.example", want: false},
		{name: "null origin", origin: "null", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://api.example/ws", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if got := checker(req); got != tt.want {
				t.Fatalf("checker(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}
