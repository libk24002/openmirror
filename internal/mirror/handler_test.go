package mirror

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libk24002/openmirror/internal/cache"
)

func TestHandlerMetadataPathMissThenHit(t *testing.T) {
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

	cacheKey := buildCacheKey(firstReq)
	if _, ok, err := c.Get(cacheKey); err != nil {
		t.Fatalf("cache get returned error: %v", err)
	} else if !ok {
		t.Fatal("expected metadata response to be cached")
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

func TestHandlerForwardsHEADMethod(t *testing.T) {
	var upstreamHits atomic.Int32
	var gotMethod string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	c := cache.NewFSCache(t.TempDir())
	h := NewHandler(c, upstream.URL, time.Minute)

	req := httptest.NewRequest(http.MethodHead, "/v2/library/alpine/manifests/latest", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotMethod != http.MethodHead {
		t.Fatalf("upstream method = %q, want %q", gotMethod, http.MethodHead)
	}
	if got := upstreamHits.Load(); got != 1 {
		t.Fatalf("upstream hit count = %d, want %d", got, 1)
	}
}

func TestHandlerForwardsQueryString(t *testing.T) {
	querySeen := ""

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		querySeen = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	c := cache.NewFSCache(t.TempDir())
	h := NewHandler(c, upstream.URL, time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/tags/list?n=5&last=sha256%3Aabc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if querySeen != "n=5&last=sha256%3Aabc" {
		t.Fatalf("upstream query = %q, want %q", querySeen, "n=5&last=sha256%3Aabc")
	}
}

func TestHandlerForwardsAcceptAndAuthorizationHeaders(t *testing.T) {
	acceptSeen := ""
	authorizationSeen := ""

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptSeen = r.Header.Get("Accept")
		authorizationSeen = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	c := cache.NewFSCache(t.TempDir())
	h := NewHandler(c, upstream.URL, time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest", nil)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if acceptSeen != "application/vnd.docker.distribution.manifest.v2+json" {
		t.Fatalf("upstream accept = %q, want %q", acceptSeen, "application/vnd.docker.distribution.manifest.v2+json")
	}
	if authorizationSeen != "Bearer test-token" {
		t.Fatalf("upstream authorization = %q, want %q", authorizationSeen, "Bearer test-token")
	}
}

func TestHandlerCacheKeyVariesByMethodQueryAndAccept(t *testing.T) {
	var upstreamHits atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		_, _ = w.Write([]byte(strings.Join([]string{r.Method, r.URL.RawQuery, r.Header.Get("Accept")}, "|")))
	}))
	defer upstream.Close()

	c := cache.NewFSCache(t.TempDir())
	h := NewHandler(c, upstream.URL, time.Minute)

	firstReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest?ref=latest", nil)
	firstReq.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	firstRec := httptest.NewRecorder()
	h.ServeHTTP(firstRec, firstReq)

	secondReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest?ref=latest", nil)
	secondReq.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	secondRec := httptest.NewRecorder()
	h.ServeHTTP(secondRec, secondReq)

	thirdReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest?ref=latest", nil)
	thirdReq.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")
	thirdRec := httptest.NewRecorder()
	h.ServeHTTP(thirdRec, thirdReq)

	fourthReq := httptest.NewRequest(http.MethodHead, "/v2/library/alpine/manifests/latest?ref=latest", nil)
	fourthReq.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")
	fourthRec := httptest.NewRecorder()
	h.ServeHTTP(fourthRec, fourthReq)

	if firstRec.Body.String() != secondRec.Body.String() {
		t.Fatalf("expected identical cache hit body, got %q and %q", firstRec.Body.String(), secondRec.Body.String())
	}
	if thirdRec.Body.String() == secondRec.Body.String() {
		t.Fatalf("expected varied Accept to bypass cache, got %q", thirdRec.Body.String())
	}
	if fourthRec.Code != http.StatusOK {
		t.Fatalf("fourth status = %d, want %d", fourthRec.Code, http.StatusOK)
	}
	if got := upstreamHits.Load(); got != 3 {
		t.Fatalf("upstream hit count = %d, want %d", got, 3)
	}
}

func TestHandlerDoesNotCacheUpstreamServerErrors(t *testing.T) {
	var upstreamHits atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"temporary failure"}`))
	}))
	defer upstream.Close()

	c := cache.NewFSCache(t.TempDir())
	h := NewHandler(c, upstream.URL, time.Hour)

	firstReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest", nil)
	firstRec := httptest.NewRecorder()
	h.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusInternalServerError {
		t.Fatalf("first status = %d, want %d", firstRec.Code, http.StatusInternalServerError)
	}
	if got := firstRec.Body.String(); got != `{"error":"temporary failure"}` {
		t.Fatalf("first body = %q, want %q", got, `{"error":"temporary failure"}`)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest", nil)
	secondRec := httptest.NewRecorder()
	h.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusInternalServerError {
		t.Fatalf("second status = %d, want %d", secondRec.Code, http.StatusInternalServerError)
	}
	if got := secondRec.Body.String(); got != `{"error":"temporary failure"}` {
		t.Fatalf("second body = %q, want %q", got, `{"error":"temporary failure"}`)
	}

	if got := upstreamHits.Load(); got != 2 {
		t.Fatalf("upstream hit count = %d, want %d", got, 2)
	}
}

func TestHandlerForwardsUpstreamAuthChallengeHeaders(t *testing.T) {
	challengeHeaders := []string{
		`Bearer realm="https://auth.example/token",service="registry.example.com"`,
		`Basic realm="registry.example.com"`,
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, challenge := range challengeHeaders {
			w.Header().Add("WWW-Authenticate", challenge)
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	c := cache.NewFSCache(t.TempDir())
	h := NewHandler(c, upstream.URL, time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Result().Header.Values("WWW-Authenticate"); !reflect.DeepEqual(got, challengeHeaders) {
		t.Fatalf("WWW-Authenticate = %#v, want %#v", got, challengeHeaders)
	}
}

func TestHandlerCacheHitPreservesAuthChallengeHeaders(t *testing.T) {
	var upstreamHits atomic.Int32
	challengeHeaders := []string{
		`Bearer realm="https://auth.example/token",service="registry.example.com"`,
		`Basic realm="registry.example.com"`,
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		for _, challenge := range challengeHeaders {
			w.Header().Add("WWW-Authenticate", challenge)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
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
	if got := firstRec.Result().Header.Values("WWW-Authenticate"); !reflect.DeepEqual(got, challengeHeaders) {
		t.Fatalf("first WWW-Authenticate = %#v, want %#v", got, challengeHeaders)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/manifests/latest", nil)
	secondRec := httptest.NewRecorder()
	h.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d", secondRec.Code, http.StatusOK)
	}
	if got := secondRec.Result().Header.Values("WWW-Authenticate"); !reflect.DeepEqual(got, challengeHeaders) {
		t.Fatalf("second WWW-Authenticate = %#v, want %#v", got, challengeHeaders)
	}
	if got := upstreamHits.Load(); got != 1 {
		t.Fatalf("upstream hit count = %d, want %d", got, 1)
	}
}

func TestHandlerRangeRequestsAreNotCached(t *testing.T) {
	var upstreamHits atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		if got := r.Header.Get("Range"); got != "bytes=0-9" {
			t.Fatalf("upstream range = %q, want %q", got, "bytes=0-9")
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("partial"))
	}))
	defer upstream.Close()

	c := cache.NewFSCache(t.TempDir())
	h := NewHandler(c, upstream.URL, time.Hour)

	firstReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/blobs/sha256:abc", nil)
	firstReq.Header.Set("Range", "bytes=0-9")
	firstRec := httptest.NewRecorder()
	h.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusPartialContent {
		t.Fatalf("first status = %d, want %d", firstRec.Code, http.StatusPartialContent)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/blobs/sha256:abc", nil)
	secondReq.Header.Set("Range", "bytes=0-9")
	secondRec := httptest.NewRecorder()
	h.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusPartialContent {
		t.Fatalf("second status = %d, want %d", secondRec.Code, http.StatusPartialContent)
	}

	if got := upstreamHits.Load(); got != 2 {
		t.Fatalf("upstream hit count = %d, want %d", got, 2)
	}
}

func TestHandlerDoesNotCache206Responses(t *testing.T) {
	var upstreamHits atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("partial"))
	}))
	defer upstream.Close()

	c := cache.NewFSCache(t.TempDir())
	h := NewHandler(c, upstream.URL, time.Hour)

	firstReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/blobs/sha256:def", nil)
	firstRec := httptest.NewRecorder()
	h.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusPartialContent {
		t.Fatalf("first status = %d, want %d", firstRec.Code, http.StatusPartialContent)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/blobs/sha256:def", nil)
	secondRec := httptest.NewRecorder()
	h.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusPartialContent {
		t.Fatalf("second status = %d, want %d", secondRec.Code, http.StatusPartialContent)
	}

	if got := upstreamHits.Load(); got != 2 {
		t.Fatalf("upstream hit count = %d, want %d", got, 2)
	}
}

func TestHandlerLargeArtifactPathsBypassCache(t *testing.T) {
	var upstreamHits atomic.Int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("blob-bytes"))
	}))
	defer upstream.Close()

	c := cache.NewFSCache(t.TempDir())
	h := NewHandler(c, upstream.URL, time.Hour)

	firstReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/blobs/sha256:abc", nil)
	firstRec := httptest.NewRecorder()
	h.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", firstRec.Code, http.StatusOK)
	}
	if got := firstRec.Body.String(); got != "blob-bytes" {
		t.Fatalf("first body = %q, want %q", got, "blob-bytes")
	}

	cacheKey := buildCacheKey(firstReq)
	if _, ok, err := c.Get(cacheKey); err != nil {
		t.Fatalf("cache get returned error: %v", err)
	} else if ok {
		t.Fatal("expected large artifact response to bypass cache write")
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/v2/library/alpine/blobs/sha256:abc", nil)
	secondRec := httptest.NewRecorder()
	h.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d", secondRec.Code, http.StatusOK)
	}
	if got := secondRec.Body.String(); got != "blob-bytes" {
		t.Fatalf("second body = %q, want %q", got, "blob-bytes")
	}

	if got := upstreamHits.Load(); got != 2 {
		t.Fatalf("upstream hit count = %d, want %d", got, 2)
	}
}
