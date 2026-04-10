package main

import (
	"log"
	"net/http"

	"github.com/libk24002/openmirror/internal/server"
)

func main() {
	log.Fatal(http.ListenAndServe(":8080", server.NewRouter()))
}
