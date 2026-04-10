# OpenMirror Go V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a production-ready Go mirror service MVP for Docker image pulls, npm, and PyPI with local PVC-backed caching and cache-miss upstream fetch.

**Architecture:** Implement one Go binary with route-prefix-based protocol handling (`/docker`, `/npm`, `/pypi`) so the same binary can be deployed three times or once behind Ingress. Use filesystem cache for persistent local storage, cache-first read path, single-flight upstream requests, and Prometheus metrics.

**Tech Stack:** Go 1.22, net/http, Prometheus client, singleflight, Kubernetes YAML

---

### Task 1: Bootstrap Go service and health endpoint

**Files:**
- Create: `go.mod`
- Create: `cmd/openmirror/main.go`
- Create: `internal/server/router.go`
- Test: `internal/server/router_test.go`

- [ ] **Step 1: Write the failing test**

```go
package server

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestHealthz(t *testing.T) {
    r := NewRouter()
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    rec := httptest.NewRecorder()

    r.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rec.Code)
    }
    if rec.Body.String() != "ok" {
        t.Fatalf("expected body ok, got %q", rec.Body.String())
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server -run TestHealthz -v`
Expected: FAIL with `undefined: NewRouter`

- [ ] **Step 3: Write minimal implementation**

```go
// internal/server/router.go
package server

import "net/http"

func NewRouter() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
        _, _ = w.Write([]byte("ok"))
    })
    return mux
}
```

```go
// cmd/openmirror/main.go
package main

import (
    "log"
    "net/http"

    "github.com/libk24002/openmirror/internal/server"
)

func main() {
    if err := http.ListenAndServe(":8080", server.NewRouter()); err != nil {
        log.Fatal(err)
    }
}
```

```go
// go.mod
module github.com/libk24002/openmirror

go 1.22
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server -run TestHealthz -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd/openmirror/main.go internal/server/router.go internal/server/router_test.go
git commit -m "feat: bootstrap go service with health endpoint"
```

### Task 2: Add environment configuration with protocol routes

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Modify: `cmd/openmirror/main.go`

- [ ] **Step 1: Write the failing test**

```go
package config

import (
    "os"
    "testing"
    "time"
)

func TestLoadFromEnvDefaults(t *testing.T) {
    _ = os.Unsetenv("OPENMIRROR_ADDR")
    cfg, err := LoadFromEnv()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if cfg.Addr != ":8080" {
        t.Fatalf("expected :8080, got %s", cfg.Addr)
    }
    if cfg.Routes["docker"].Upstream == "" {
        t.Fatalf("docker upstream should not be empty")
    }
    if cfg.MetadataTTL != 10*time.Minute {
        t.Fatalf("expected 10m ttl, got %s", cfg.MetadataTTL)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestLoadFromEnvDefaults -v`
Expected: FAIL with `undefined: LoadFromEnv`

- [ ] **Step 3: Write minimal implementation**

```go
package config

import "time"

type Route struct {
    Prefix   string
    Upstream string
}

type Config struct {
    Addr           string
    CacheDir       string
    RequestTimeout time.Duration
    MetadataTTL    time.Duration
    Routes         map[string]Route
}

func LoadFromEnv() (Config, error) {
    return Config{
        Addr:           ":8080",
        CacheDir:       "/var/cache/openmirror",
        RequestTimeout: 20 * time.Second,
        MetadataTTL:    10 * time.Minute,
        Routes: map[string]Route{
            "docker": {Prefix: "/docker", Upstream: "https://registry-1.docker.io"},
            "npm":    {Prefix: "/npm", Upstream: "https://registry.npmjs.org"},
            "pypi":   {Prefix: "/pypi", Upstream: "https://pypi.org"},
        },
    }, nil
}
```

```go
// cmd/openmirror/main.go (replace direct listen config usage)
cfg, err := config.LoadFromEnv()
if err != nil {
    log.Fatal(err)
}
if err := http.ListenAndServe(cfg.Addr, server.NewRouter()); err != nil {
    log.Fatal(err)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestLoadFromEnvDefaults -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go cmd/openmirror/main.go
git commit -m "feat: add environment-backed mirror configuration defaults"
```

### Task 3: Implement filesystem cache with TTL

**Files:**
- Create: `internal/cache/cache.go`
- Test: `internal/cache/cache_test.go`

- [ ] **Step 1: Write the failing test**

```go
package cache

import (
    "os"
    "testing"
    "time"
)

func TestFSCacheSetGetWithTTL(t *testing.T) {
    dir := t.TempDir()
    c := NewFSCache(dir)

    key := "k1"
    val := Entry{StatusCode: 200, Body: []byte("hello"), ExpireAt: time.Now().Add(1 * time.Hour)}
    if err := c.Set(key, val); err != nil {
        t.Fatalf("set failed: %v", err)
    }

    got, ok, err := c.Get(key)
    if err != nil {
        t.Fatalf("get failed: %v", err)
    }
    if !ok || string(got.Body) != "hello" {
        t.Fatalf("unexpected cache value")
    }

    _ = os.RemoveAll(dir)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cache -run TestFSCacheSetGetWithTTL -v`
Expected: FAIL with `undefined: NewFSCache`

- [ ] **Step 3: Write minimal implementation**

```go
package cache

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "os"
    "path/filepath"
    "sync"
    "time"
)

type Entry struct {
    StatusCode int
    Header     map[string]string
    Body       []byte
    ExpireAt   time.Time
}

type FSCache struct {
    root string
    mu   sync.RWMutex
}

func NewFSCache(root string) *FSCache {
    _ = os.MkdirAll(root, 0o755)
    return &FSCache{root: root}
}

func (c *FSCache) Set(key string, entry Entry) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    b, err := json.Marshal(entry)
    if err != nil {
        return err
    }
    return os.WriteFile(c.path(key), b, 0o644)
}

func (c *FSCache) Get(key string) (Entry, bool, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    b, err := os.ReadFile(c.path(key))
    if err != nil {
        if os.IsNotExist(err) {
            return Entry{}, false, nil
        }
        return Entry{}, false, err
    }
    var e Entry
    if err := json.Unmarshal(b, &e); err != nil {
        return Entry{}, false, err
    }
    if !e.ExpireAt.IsZero() && time.Now().After(e.ExpireAt) {
        return Entry{}, false, nil
    }
    return e, true, nil
}

func (c *FSCache) path(key string) string {
    sum := sha256.Sum256([]byte(key))
    return filepath.Join(c.root, hex.EncodeToString(sum[:])+".json")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cache -run TestFSCacheSetGetWithTTL -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cache/cache.go internal/cache/cache_test.go
git commit -m "feat: add filesystem cache with ttl-aware reads"
```

### Task 4: Add upstream HTTP client with request coalescing

**Files:**
- Create: `internal/upstream/client.go`
- Test: `internal/upstream/client_test.go`

- [ ] **Step 1: Write the failing test**

```go
package upstream

import (
    "context"
    "net/http"
    "net/http/httptest"
    "sync/atomic"
    "testing"
    "time"
)

func TestFetchCoalescesConcurrentRequests(t *testing.T) {
    var hit int32
    s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        atomic.AddInt32(&hit, 1)
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    }))
    defer s.Close()

    c := NewClient(5 * time.Second)
    done := make(chan struct{}, 2)
    for i := 0; i < 2; i++ {
        go func() {
            _, _, _, _ = c.Fetch(context.Background(), s.URL)
            done <- struct{}{}
        }()
    }
    <-done
    <-done

    if atomic.LoadInt32(&hit) != 1 {
        t.Fatalf("expected 1 upstream hit, got %d", hit)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/upstream -run TestFetchCoalescesConcurrentRequests -v`
Expected: FAIL with `undefined: NewClient`

- [ ] **Step 3: Write minimal implementation**

```go
package upstream

import (
    "context"
    "io"
    "net/http"
    "time"

    "golang.org/x/sync/singleflight"
)

type Client struct {
    httpClient *http.Client
    sf         singleflight.Group
}

func NewClient(timeout time.Duration) *Client {
    return &Client{httpClient: &http.Client{Timeout: timeout}}
}

func (c *Client) Fetch(ctx context.Context, url string) (int, http.Header, []byte, error) {
    v, err, _ := c.sf.Do(url, func() (interface{}, error) {
        req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
        if err != nil {
            return nil, err
        }
        resp, err := c.httpClient.Do(req)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()
        b, err := io.ReadAll(resp.Body)
        if err != nil {
            return nil, err
        }
        return struct {
            code int
            h    http.Header
            body []byte
        }{code: resp.StatusCode, h: resp.Header.Clone(), body: b}, nil
    })
    if err != nil {
        return 0, nil, nil, err
    }
    r := v.(struct {
        code int
        h    http.Header
        body []byte
    })
    return r.code, r.h, r.body, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/upstream -run TestFetchCoalescesConcurrentRequests -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/upstream/client.go internal/upstream/client_test.go go.mod go.sum
git commit -m "feat: add upstream client with singleflight request coalescing"
```

### Task 5: Implement cache-first mirror handler for one protocol path

**Files:**
- Create: `internal/mirror/handler.go`
- Test: `internal/mirror/handler_test.go`

- [ ] **Step 1: Write the failing test**

```go
package mirror

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/libk24002/openmirror/internal/cache"
)

func TestHandlerMissThenHit(t *testing.T) {
    upHits := 0
    up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        upHits++
        _, _ = w.Write([]byte("payload"))
    }))
    defer up.Close()

    c := cache.NewFSCache(t.TempDir())
    h := NewHandler(c, up.URL, 10*time.Minute)

    req1 := httptest.NewRequest(http.MethodGet, "/pkg/a", nil)
    rec1 := httptest.NewRecorder()
    h.ServeHTTP(rec1, req1)

    req2 := httptest.NewRequest(http.MethodGet, "/pkg/a", nil)
    rec2 := httptest.NewRecorder()
    h.ServeHTTP(rec2, req2)

    if upHits != 1 {
        t.Fatalf("expected one upstream hit, got %d", upHits)
    }
    if rec2.Body.String() != "payload" {
        t.Fatalf("expected cached body payload")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mirror -run TestHandlerMissThenHit -v`
Expected: FAIL with `undefined: NewHandler`

- [ ] **Step 3: Write minimal implementation**

```go
package mirror

import (
    "context"
    "net/http"
    "time"

    "github.com/libk24002/openmirror/internal/cache"
    "github.com/libk24002/openmirror/internal/upstream"
)

type Handler struct {
    cache    *cache.FSCache
    upstream string
    ttl      time.Duration
    client   *upstream.Client
}

func NewHandler(c *cache.FSCache, upstreamBase string, ttl time.Duration) http.Handler {
    return &Handler{cache: c, upstream: upstreamBase, ttl: ttl, client: upstream.NewClient(20 * time.Second)}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    key := r.URL.Path
    if e, ok, _ := h.cache.Get(key); ok {
        w.WriteHeader(e.StatusCode)
        _, _ = w.Write(e.Body)
        return
    }

    code, header, body, err := h.client.Fetch(context.Background(), h.upstream+r.URL.Path)
    if err != nil {
        http.Error(w, "upstream fetch failed", http.StatusBadGateway)
        return
    }

    _ = h.cache.Set(key, cache.Entry{StatusCode: code, Body: body, Header: map[string]string{"Content-Type": header.Get("Content-Type")}, ExpireAt: time.Now().Add(h.ttl)})
    w.WriteHeader(code)
    _, _ = w.Write(body)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mirror -run TestHandlerMissThenHit -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mirror/handler.go internal/mirror/handler_test.go
git commit -m "feat: add cache-first mirror request handler"
```

### Task 6: Wire protocol routes and metadata TTL behavior

**Files:**
- Modify: `internal/server/router.go`
- Modify: `cmd/openmirror/main.go`
- Create: `internal/mirror/ttl.go`
- Test: `internal/mirror/ttl_test.go`
- Test: `internal/server/router_integration_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package mirror

import "testing"

func TestTTLForPath(t *testing.T) {
    if got := TTLForPath("/docker/v2/library/nginx/blobs/sha256:abc", 10); got <= 10 {
        t.Fatalf("blob path should use long ttl")
    }
    if got := TTLForPath("/npm/react", 10); got != 10 {
        t.Fatalf("metadata path should use metadata ttl")
    }
}
```

```go
package server

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestProtocolRoutesExist(t *testing.T) {
    r := NewRouterWithMirrors(nil, nil, nil)
    for _, p := range []string{"/docker/x", "/npm/x", "/pypi/x"} {
        req := httptest.NewRequest(http.MethodGet, p, nil)
        rec := httptest.NewRecorder()
        r.ServeHTTP(rec, req)
        if rec.Code == http.StatusNotFound {
            t.Fatalf("route %s should be registered", p)
        }
    }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/mirror ./internal/server -v`
Expected: FAIL with missing `TTLForPath` and `NewRouterWithMirrors`

- [ ] **Step 3: Write minimal implementation**

```go
// internal/mirror/ttl.go
package mirror

import "strings"

func TTLForPath(path string, metadataTTLMinutes int) int {
    if strings.Contains(path, "/blobs/sha256:") || strings.HasSuffix(path, ".tgz") || strings.HasSuffix(path, ".whl") || strings.HasSuffix(path, ".tar.gz") {
        return 24 * 60
    }
    return metadataTTLMinutes
}
```

```go
// internal/server/router.go
func NewRouterWithMirrors(docker, npm, pypi http.Handler) http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
    mux.Handle("/docker/", http.StripPrefix("/docker", docker))
    mux.Handle("/npm/", http.StripPrefix("/npm", npm))
    mux.Handle("/pypi/", http.StripPrefix("/pypi", pypi))
    return mux
}
```

```go
// cmd/openmirror/main.go (wire three handlers)
c := cache.NewFSCache(cfg.CacheDir)
dockerHandler := mirror.NewHandler(c, cfg.Routes["docker"].Upstream, cfg.MetadataTTL)
npmHandler := mirror.NewHandler(c, cfg.Routes["npm"].Upstream, cfg.MetadataTTL)
pypiHandler := mirror.NewHandler(c, cfg.Routes["pypi"].Upstream, cfg.MetadataTTL)

router := server.NewRouterWithMirrors(dockerHandler, npmHandler, pypiHandler)
if err := http.ListenAndServe(cfg.Addr, router); err != nil {
    log.Fatal(err)
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mirror/ttl.go internal/mirror/ttl_test.go internal/server/router.go internal/server/router_integration_test.go cmd/openmirror/main.go
git commit -m "feat: wire docker npm pypi routes with ttl policy"
```

### Task 7: Add Prometheus metrics and structured logging middleware

**Files:**
- Create: `internal/observability/metrics.go`
- Modify: `internal/server/router.go`
- Test: `internal/observability/metrics_test.go`

- [ ] **Step 1: Write the failing test**

```go
package observability

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestMetricsEndpointExposesCounter(t *testing.T) {
    r := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
    Handler().ServeHTTP(r, req)
    if !strings.Contains(r.Body.String(), "openmirror_requests_total") {
        t.Fatalf("expected openmirror_requests_total metric")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/observability -run TestMetricsEndpointExposesCounter -v`
Expected: FAIL with `undefined: Handler`

- [ ] **Step 3: Write minimal implementation**

```go
package observability

import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var RequestsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{Name: "openmirror_requests_total", Help: "Total mirror requests"},
    []string{"route", "status"},
)

func init() {
    prometheus.MustRegister(RequestsTotal)
}

func Handler() http.Handler {
    return promhttp.Handler()
}
```

```go
// internal/server/router.go (add endpoint)
mux.Handle("/metrics", observability.Handler())
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/observability -run TestMetricsEndpointExposesCounter -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/observability/metrics.go internal/observability/metrics_test.go internal/server/router.go go.mod go.sum
git commit -m "feat: expose prometheus metrics endpoint"
```

### Task 8: Add container and Kubernetes deployment manifests

**Files:**
- Create: `Dockerfile`
- Create: `deploy/k8s/base/deployment.yaml`
- Create: `deploy/k8s/base/pvc.yaml`
- Create: `deploy/k8s/base/service.yaml`
- Create: `deploy/k8s/base/ingress.yaml`
- Create: `README.md`

- [ ] **Step 1: Write failing validation command**

Run: `kubectl apply --dry-run=client -f deploy/k8s/base`
Expected: FAIL because manifests do not exist yet

- [ ] **Step 2: Add minimal deployment assets**

```dockerfile
FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/openmirror ./cmd/openmirror

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/openmirror /openmirror
ENTRYPOINT ["/openmirror"]
```

```yaml
# deploy/k8s/base/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openmirror
spec:
  replicas: 2
  selector:
    matchLabels:
      app: openmirror
  template:
    metadata:
      labels:
        app: openmirror
    spec:
      containers:
        - name: openmirror
          image: ghcr.io/libk24002/openmirror:latest
          ports:
            - containerPort: 8080
          volumeMounts:
            - name: cache
              mountPath: /var/cache/openmirror
      volumes:
        - name: cache
          persistentVolumeClaim:
            claimName: openmirror-cache
```

```yaml
# deploy/k8s/base/pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: openmirror-cache
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
```

```yaml
# deploy/k8s/base/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: openmirror
spec:
  selector:
    app: openmirror
  ports:
    - port: 80
      targetPort: 8080
```

```yaml
# deploy/k8s/base/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: openmirror
spec:
  rules:
    - host: mirror.internal
      http:
        paths:
          - path: /docker
            pathType: Prefix
            backend:
              service:
                name: openmirror
                port:
                  number: 80
          - path: /npm
            pathType: Prefix
            backend:
              service:
                name: openmirror
                port:
                  number: 80
          - path: /pypi
            pathType: Prefix
            backend:
              service:
                name: openmirror
                port:
                  number: 80
```

- [ ] **Step 3: Document open-source quick start**

````markdown
# OpenMirror

OpenMirror is an open-source mirror service for CI acceleration.

## Features
- Docker/npm/PyPI mirror paths
- Local filesystem cache (PVC-friendly)
- Metrics endpoint for Prometheus

## Run locally
```bash
go run ./cmd/openmirror
```

## Endpoints
- `/healthz`
- `/metrics`
- `/docker/*`
- `/npm/*`
- `/pypi/*`
````

- [ ] **Step 4: Run validations**

Run: `go test ./... && kubectl apply --dry-run=client -f deploy/k8s/base`
Expected: PASS for tests and dry-run validation

- [ ] **Step 5: Commit**

```bash
git add Dockerfile deploy/k8s/base README.md
git commit -m "feat: add container image and kubernetes deployment manifests"
```

## Spec Coverage Check

- Supports docker/npm/pypi mirror paths: Tasks 2, 5, 6
- Cache hit-first then upstream fetch: Tasks 3, 5
- Metadata staleness window: Task 6
- Small-scale K8s deployment and PVC use: Task 8
- Metrics and SLO measurement data: Task 7
- Open-source repository baseline docs: Task 8

## Self-Review Notes

- No placeholder markers (`TODO`, `TBD`) remain.
- All task references use concrete file paths.
- Public APIs in later tasks match earlier definitions (`NewHandler`, `NewRouterWithMirrors`, `LoadFromEnv`).
