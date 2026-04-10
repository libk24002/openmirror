package mirror

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/libk24002/openmirror/internal/cache"
)

type handler struct {
	cache        *cache.FSCache
	upstreamBase string
	ttl          time.Duration
}

type cachedResponse struct {
	Status  int         `json:"status"`
	Headers http.Header `json:"headers"`
	Body    []byte      `json:"body"`
}

func NewHandler(c *cache.FSCache, upstreamBase string, ttl time.Duration) http.Handler {
	return &handler{
		cache:        c,
		upstreamBase: strings.TrimRight(upstreamBase, "/"),
		ttl:          ttl,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path

	if entry, ok, err := h.cache.Get(key); err == nil && ok {
		var cached cachedResponse
		if err := json.Unmarshal(entry.Value, &cached); err == nil {
			writeResponse(w, cached)
			return
		}
	}

	upstreamURL := h.upstreamBase + key
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}

	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}

	cached := cachedResponse{
		Status:  resp.StatusCode,
		Headers: resp.Header.Clone(),
		Body:    body,
	}

	if serialized, err := json.Marshal(cached); err == nil {
		_ = h.cache.Set(key, cache.Entry{
			Value:    serialized,
			ExpireAt: time.Now().Add(h.ttl),
		})
	}

	writeResponse(w, cached)
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
