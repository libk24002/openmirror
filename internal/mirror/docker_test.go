package mirror

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDockerCompatHandlerRewritesDockerIOPath(t *testing.T) {
	var gotPath string
	handler := NewDockerCompatHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v2/docker.io/library/nginx/manifests/latest", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotPath != "/v2/library/nginx/manifests/latest" {
		t.Fatalf("rewritten path = %q, want %q", gotPath, "/v2/library/nginx/manifests/latest")
	}
}

func TestDockerCompatHandlerKeepsNonDockerIOPath(t *testing.T) {
	var gotPath string
	handler := NewDockerCompatHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v2/library/nginx/manifests/latest", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotPath != "/v2/library/nginx/manifests/latest" {
		t.Fatalf("path = %q, want %q", gotPath, "/v2/library/nginx/manifests/latest")
	}
}
