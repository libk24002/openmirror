package mirror

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/libk24002/openmirror/internal/cache"
	"github.com/libk24002/openmirror/internal/upstream"
)

const defaultUpstreamTimeout = 20 * time.Second

var hopByHopResponseHeaders = map[string]struct{}{
	"Connection":          {},
	"Proxy-Connection":    {},
	"Keep-Alive":          {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
	"TE":                  {},
	"Trailer":             {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
}

type handler struct {
	cache          *cache.FSCache
	upstreamBase   string
	ttl            time.Duration
	upstreamClient *upstream.Client
}

type cachedResponse struct {
	Status  int         `json:"status"`
	Headers http.Header `json:"headers"`
	Body    []byte      `json:"body"`
}

func NewHandler(c *cache.FSCache, upstreamBase string, ttl time.Duration) http.Handler {
	return NewHandlerWithClient(c, upstream.NewClient(defaultUpstreamTimeout), upstreamBase, ttl)
}

func NewHandlerWithClient(c *cache.FSCache, client *upstream.Client, upstreamBase string, ttl time.Duration) http.Handler {
	if client == nil {
		client = upstream.NewClient(defaultUpstreamTimeout)
	}

	return &handler{
		cache:          c,
		upstreamBase:   strings.TrimRight(upstreamBase, "/"),
		ttl:            ttl,
		upstreamClient: client,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cacheKey := buildCacheKey(r)
	cacheable := isCacheableMethod(r.Method) && !hasRangeHeader(r.Header)
	largeArtifactPath := IsLargeArtifactPath(r.URL.Path)

	if cacheable && !largeArtifactPath {
		if entry, ok, err := h.cache.Get(cacheKey); err == nil && ok {
			var cached cachedResponse
			if err := json.Unmarshal(entry.Value, &cached); err == nil {
				writeResponse(w, cached)
				return
			}
		}
	}

	upstreamURL := h.upstreamBase + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	if largeArtifactPath {
		upstreamResp, err := h.upstreamClient.DoRequest(r.Context(), upstream.Request{
			Method:  r.Method,
			URL:     upstreamURL,
			Headers: requestHeadersForUpstream(r.Header),
		})
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		defer upstreamResp.Body.Close()

		for key, values := range responseHeadersForDownstream(upstreamResp.Header) {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		w.WriteHeader(upstreamResp.StatusCode)
		_, _ = io.Copy(w, upstreamResp.Body)
		return
	}

	statusCode, headers, body, err := h.upstreamClient.FetchRequest(r.Context(), upstream.Request{
		Method:  r.Method,
		URL:     upstreamURL,
		Headers: requestHeadersForUpstream(r.Header),
	})
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}

	cached := cachedResponse{
		Status:  statusCode,
		Headers: responseHeadersForDownstream(headers),
		Body:    body,
	}

	if cacheable && isCacheableStatus(statusCode) {
		if serialized, err := json.Marshal(cached); err == nil {
			ttlMinutes := TTLForPath(r.URL.Path, int(h.ttl/time.Minute))
			_ = h.cache.Set(cacheKey, cache.Entry{
				Value:    serialized,
				ExpireAt: time.Now().Add(time.Duration(ttlMinutes) * time.Minute),
			})
		}
	}

	writeResponse(w, cached)
}

func isCacheableMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

func isCacheableStatus(statusCode int) bool {
	if statusCode == http.StatusPartialContent {
		return false
	}

	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

func hasRangeHeader(headers http.Header) bool {
	return strings.TrimSpace(headers.Get("Range")) != ""
}

func buildCacheKey(r *http.Request) string {
	authorization := r.Header.Values("Authorization")
	authorizationHash := "none"
	if len(authorization) > 0 {
		hash := sha256.New()
		for _, value := range authorization {
			_, _ = hash.Write([]byte(value))
			_, _ = hash.Write([]byte{0})
		}
		authorizationHash = hex.EncodeToString(hash.Sum(nil))
	}

	return strings.Join([]string{
		r.Method,
		r.URL.Path,
		r.URL.RawQuery,
		r.Header.Get("Accept"),
		authorizationHash,
	}, "\n")
}

func requestHeadersForUpstream(headers http.Header) http.Header {
	cloned := make(http.Header, len(headers))
	for key, values := range headers {
		if strings.EqualFold(key, "Host") {
			continue
		}
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

func responseHeadersForDownstream(headers http.Header) http.Header {
	forwarded := make(http.Header, len(headers))
	connectionScopedHeaders := parseConnectionHeaderTokens(headers)

	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		if isHopByHopHeader(key) {
			continue
		}
		if _, listedInConnection := connectionScopedHeaders[http.CanonicalHeaderKey(key)]; listedInConnection {
			continue
		}

		forwarded[key] = append([]string(nil), values...)
	}

	return forwarded
}

func isHopByHopHeader(headerName string) bool {
	_, ok := hopByHopResponseHeaders[http.CanonicalHeaderKey(headerName)]
	return ok
}

func parseConnectionHeaderTokens(headers http.Header) map[string]struct{} {
	connectionTokens := make(map[string]struct{})
	for _, value := range headers.Values("Connection") {
		tokens := strings.Split(value, ",")
		for _, token := range tokens {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			connectionTokens[http.CanonicalHeaderKey(token)] = struct{}{}
		}
	}

	return connectionTokens
}

func writeResponse(w http.ResponseWriter, response cachedResponse) {
	for key, values := range response.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(response.Status)
	_, _ = w.Write(response.Body)
}
