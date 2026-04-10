package mirror

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libk24002/openmirror/internal/cache"
)

func TestHandlerMissThenHit(t *testing.T) {
	var upstreamHits atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"library/alpine"}`))
	}))
	defer upstream.Close()

	c := cache.NewFSCache(t.TempDir())
	h := NewHandler(c, upstream.URL, time.Minute)

	firstReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest", nil)
	firstRec := httptest.NewRecorder()
	h.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", firstRec.Code, http.StatusOK)
	}
	if got := firstRec.Body.String(); got != `{"name":"library/alpine"}` {
		t.Fatalf("first body = %q, want %q", got, `{"name":"library/alpine"}`)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest", nil)
	secondRec := httptest.NewRecorder()
	h.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d", secondRec.Code, http.StatusOK)
	}
	if got := secondRec.Body.String(); got != `{"name":"library/alpine"}` {
		t.Fatalf("second body = %q, want %q", got, `{"name":"library/alpine"}`)
	}

	if got := upstreamHits.Load(); got != 1 {
		t.Fatalf("upstream hit count = %d, want %d", got, 1)
	}
}
