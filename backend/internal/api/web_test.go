package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebServingCacheAndAssetFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<div id=app></div>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app-123.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	s := &Server{WebDir: dir, Secret: []byte("test-secret")}
	tests := []struct {
		path    string
		status  int
		cache   string
		body    string
		notBody string
	}{
		{
			path:   "/facturas",
			status: http.StatusOK,
			cache:  "no-store",
			body:   `<div id=app>`,
		},
		{
			path:   "/index.html",
			status: http.StatusOK,
			cache:  "no-store",
			body:   `<div id=app>`,
		},
		{
			path:   "/assets/app-123.js",
			status: http.StatusOK,
			cache:  "public, max-age=31536000, immutable",
			body:   "console.log(1)",
		},
		{
			path:    "/assets/old-chunk.js",
			status:  http.StatusNotFound,
			cache:   "no-store",
			notBody: `<div id=app>`,
		},
		{
			path:    "/favicon.ico",
			status:  http.StatusNotFound,
			cache:   "no-store",
			notBody: `<div id=app>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.handleWeb(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.cache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cache)
			}
			if tt.body != "" && !strings.Contains(rec.Body.String(), tt.body) {
				t.Errorf("body does not contain %q: %q", tt.body, rec.Body.String())
			}
			if tt.notBody != "" && strings.Contains(rec.Body.String(), tt.notBody) {
				t.Errorf("body unexpectedly contains %q", tt.notBody)
			}
		})
	}
}
