package main

import (
	"log"
	"net/http"

	"github.com/libk24002/openmirror/internal/config"
	"github.com/libk24002/openmirror/internal/server"
)

func main() {
	cfg := config.LoadFromEnv()
	log.Fatal(http.ListenAndServe(cfg.Addr, server.NewRouter()))
}
