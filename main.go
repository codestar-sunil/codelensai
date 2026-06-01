package main

import (
	"log"
	"net/http"

	"codelensai/internal/config"
	"codelensai/internal/webhook"
)

func main() {
	cfg := config.Load()

	handler := webhook.NewHandler(cfg)

	http.HandleFunc("/webhook/github", handler.Handle)

	log.Printf("CodeLens AI server running on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
