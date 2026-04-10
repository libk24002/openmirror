package mirror

import (
	"net/http"
	"strings"
)

type dockerCompatHandler struct {
	next http.Handler
}

func NewDockerCompatHandler(next http.Handler) http.Handler {
	return &dockerCompatHandler{next: nonNilHandler(next)}
}

func (h *dockerCompatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rewrittenPath := rewriteDockerPath(r.URL.Path)
	if rewrittenPath == r.URL.Path {
		h.next.ServeHTTP(w, r)
		return
	}

	rewritten := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path = rewrittenPath
	rewritten.URL = &urlCopy

	h.next.ServeHTTP(w, rewritten)
}

func rewriteDockerPath(path string) string {
	const v2Prefix = "/v2/"
	const dockerIOPrefix = "docker.io/"

	if path == "/v2/docker.io" {
		return "/v2/"
	}

	if !strings.HasPrefix(path, v2Prefix) {
		return path
	}

	rest := strings.TrimPrefix(path, v2Prefix)
	if !strings.HasPrefix(rest, dockerIOPrefix) {
		return path
	}

	return v2Prefix + strings.TrimPrefix(rest, dockerIOPrefix)
}
