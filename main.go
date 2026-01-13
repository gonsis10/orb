package main

import (
	"fmt"
	"orb/cmd"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func main() {
	loadEnv()
	cmd.Execute()
}

func loadEnv() {
	var loaded bool

	// Try ~/.config/orb/.env first
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(homeDir, ".config", "orb", ".env")
		if err := godotenv.Load(configPath); err == nil {
			loaded = true
			return
		}
	}

	// Check if required variables are already set
	if !loaded {
		requiredVars := []string{"DOMAIN", "CONFIG_PATH", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ZONE_ID", "CLOUDFLARE_ACCOUNT_ID"}
		allSet := true
		for _, v := range requiredVars {
			if os.Getenv(v) == "" {
				allSet = false
				break
			}
		}

		if !allSet {
			fmt.Fprintln(os.Stderr, "Warning: No .env file found and required environment variables are not set")
			fmt.Fprintln(os.Stderr, "Expected config at: ~/.config/orb/.env")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "To setup, run:")
			fmt.Fprintln(os.Stderr, "  mkdir -p ~/.config/orb")
			fmt.Fprintln(os.Stderr, "  nano ~/.config/orb/.env")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Required variables: DOMAIN, CONFIG_PATH, CLOUDFLARE_API_TOKEN, CLOUDFLARE_ZONE_ID, CLOUDFLARE_ACCOUNT_ID")
		} else {
			fmt.Fprintln(os.Stderr, "Using environment variables")
		}
	}
}
