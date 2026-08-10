package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoJSONSkipsOnlyNoContentResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := New(srv.URL, "")
	var out struct{}
	if err := client.doJSON(http.MethodDelete, "/", nil, "", &out); err != nil {
		t.Fatalf("doJSON() error = %v", err)
	}
}
