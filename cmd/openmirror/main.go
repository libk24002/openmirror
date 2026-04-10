package main

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/libk24002/openmirror/internal/cache"
	"github.com/libk24002/openmirror/internal/config"
	"github.com/libk24002/openmirror/internal/mirror"
	"github.com/libk24002/openmirror/internal/server"
	"github.com/libk24002/openmirror/internal/upstream"
)

func main() {
	cfg := config.LoadFromEnv()
	upstreamClient := upstream.NewClient(cfg.RequestTimeout)

	dockerHandler := mirror.NewHandlerWithClient(
		cache.NewFSCache(filepath.Join(cfg.CacheDir, "docker")),
		upstreamClient,
		cfg.Routes["docker"].Upstream,
		cfg.MetadataTTL,
	)
	npmHandler := mirror.NewHandlerWithClient(
		cache.NewFSCache(filepath.Join(cfg.CacheDir, "npm")),
		upstreamClient,
		cfg.Routes["npm"].Upstream,
		cfg.MetadataTTL,
	)
	pypiIndexHandler := mirror.NewHandlerWithClient(
		cache.NewFSCache(filepath.Join(cfg.CacheDir, "pypi")),
		upstreamClient,
		cfg.Routes["pypi"].Upstream,
		cfg.MetadataTTL,
	)
	pypiFilesHandler := mirror.NewHandlerWithClient(
		cache.NewFSCache(filepath.Join(cfg.CacheDir, "pypiFiles")),
		upstreamClient,
		cfg.Routes["pypiFiles"].Upstream,
		cfg.MetadataTTL,
	)
	pypiHandler := mirror.NewPyPIHandler(pypiIndexHandler, pypiFilesHandler)

	router := server.NewRouterWithMirrors(dockerHandler, npmHandler, pypiHandler)
	log.Fatal(http.ListenAndServe(cfg.Addr, router))
}
