package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestRequestIDGeneratesCanonicalUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/", func(c *gin.Context) {
		if RequestIDValue(c) == "" {
			t.Fatal("request ID was not stored in context")
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNoContent)
	}
	if _, err := uuid.Parse(resp.Header().Get("X-Request-ID")); err != nil {
		t.Fatalf("generated X-Request-ID = %q is invalid: %v", resp.Header().Get("X-Request-ID"), err)
	}
}

func TestRequestIDPreservesValidAndReplacesInvalidValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	valid := "4f8f6c72-9d64-4e0c-9b3d-7c7d3a9a1e11"
	validReq := httptest.NewRequest(http.MethodGet, "/", nil)
	validReq.Header.Set("X-Request-ID", valid)
	validResp := httptest.NewRecorder()
	router.ServeHTTP(validResp, validReq)
	if got := validResp.Header().Get("X-Request-ID"); got != valid {
		t.Fatalf("valid request ID = %q, want %q", got, valid)
	}

	invalidReq := httptest.NewRequest(http.MethodGet, "/", nil)
	invalidReq.Header.Set("X-Request-ID", "not-safe-for-logs")
	invalidResp := httptest.NewRecorder()
	router.ServeHTTP(invalidResp, invalidReq)
	if got := invalidResp.Header().Get("X-Request-ID"); got == "not-safe-for-logs" {
		t.Fatal("invalid request ID was echoed back")
	}
	if _, err := uuid.Parse(invalidResp.Header().Get("X-Request-ID")); err != nil {
		t.Fatalf("replacement request ID = %q is invalid: %v", invalidResp.Header().Get("X-Request-ID"), err)
	}
}
