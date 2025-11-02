package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"twentyfive/internal/app"
)

func main() {
	var (
		port     = flag.Int("port", 8080, "port to listen on")
		dataFile = flag.String("data-file", filepath.Join("data", "board.json"), "path to board data file")
	)
	flag.Parse()

	store, err := app.NewStore(*dataFile)
	if err != nil {
		log.Fatalf("initialize store: %v", err)
	}

	server := app.NewServer(store)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("TwentyFive backend listening on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}
