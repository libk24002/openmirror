package server

import (
	"net/http"

	"github.com/libk24002/openmirror/internal/observability"
)

func NewRouter() http.Handler {
	return NewRouterWithMirrors(http.NotFoundHandler(), http.NotFoundHandler(), http.NotFoundHandler())
}

func NewRouterWithMirrors(docker, npm, pypi http.Handler) http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	router.Handle("/metrics", observability.Handler())

	router.Handle("/docker/", http.StripPrefix("/docker", nonNilHandler(docker)))
	router.Handle("/npm/", http.StripPrefix("/npm", nonNilHandler(npm)))
	router.Handle("/pypi/", http.StripPrefix("/pypi", nonNilHandler(pypi)))

	return router
}

func nonNilHandler(handler http.Handler) http.Handler {
	if handler != nil {
		return handler
	}

	return http.NotFoundHandler()
}
