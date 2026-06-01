package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	GitHubAppID         string
	GitHubPrivateKey    string
	GitHubWebhookSecret string
	ClaudeAPIKey        string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading environment variables directly")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	return &Config{
		Port:                port,
		GitHubAppID:         os.Getenv("GITHUB_APP_ID"),
		GitHubPrivateKey:    strings.ReplaceAll(os.Getenv("GITHUB_PRIVATE_KEY"), "\\n", "\n"),
		GitHubWebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		ClaudeAPIKey:        os.Getenv("CLAUDE_API_KEY"),
	}
}
