package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigKey represents a known configuration key
type ConfigKey struct {
	Name        string
	Description string
	Required    bool
}

// KnownKeys lists all recognized configuration keys
var KnownKeys = []ConfigKey{
	{Name: "DOMAIN", Description: "Your domain (e.g., example.com)", Required: true},
	{Name: "CONFIG_PATH", Description: "Path to cloudflared config YAML", Required: true},
	{Name: "CLOUDFLARE_API_TOKEN", Description: "Cloudflare API token", Required: true},
	{Name: "CLOUDFLARE_ZONE_ID", Description: "Cloudflare Zone ID", Required: true},
	{Name: "CLOUDFLARE_ACCOUNT_ID", Description: "Cloudflare Account ID", Required: true},
	{Name: "USER_EMAIL", Description: "Your email (for private access)", Required: false},
}

// Service manages orb configuration
type Service struct {
	configPath string
}

// NewService creates a new config service
func NewService() (*Service, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".config", "orb", ".env")
	return &Service{configPath: configPath}, nil
}

// GetConfigPath returns the path to the config file
func (s *Service) GetConfigPath() string {
	return s.configPath
}

// EnsureConfigDir creates the config directory if it doesn't exist
func (s *Service) EnsureConfigDir() error {
	dir := filepath.Dir(s.configPath)
	return os.MkdirAll(dir, 0700)
}

// Load reads all config values from the .env file
func (s *Service) Load() (map[string]string, error) {
	config := make(map[string]string)

	file, err := os.Open(s.configPath)
	if os.IsNotExist(err) {
		return config, nil // Return empty config if file doesn't exist
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open config: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			config[key] = value
		}
	}

	return config, scanner.Err()
}

// Get retrieves a single config value
func (s *Service) Get(key string) (string, error) {
	config, err := s.Load()
	if err != nil {
		return "", err
	}

	if value, ok := config[key]; ok {
		return value, nil
	}

	// Fall back to environment variable
	return os.Getenv(key), nil
}

// Set sets a config value in the .env file
func (s *Service) Set(key, value string) error {
	if err := s.EnsureConfigDir(); err != nil {
		return err
	}

	config, err := s.Load()
	if err != nil {
		return err
	}

	config[key] = value
	return s.save(config)
}

// Unset removes a config value from the .env file
func (s *Service) Unset(key string) error {
	config, err := s.Load()
	if err != nil {
		return err
	}

	delete(config, key)
	return s.save(config)
}

// save writes the config map back to the .env file
func (s *Service) save(config map[string]string) error {
	file, err := os.Create(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	// Write known keys first in order
	written := make(map[string]bool)
	for _, known := range KnownKeys {
		if value, ok := config[known.Name]; ok {
			fmt.Fprintf(file, "%s=%s\n", known.Name, value)
			written[known.Name] = true
		}
	}

	// Write any unknown keys
	for key, value := range config {
		if !written[key] {
			fmt.Fprintf(file, "%s=%s\n", key, value)
		}
	}

	return nil
}

// List prints all config values
func (s *Service) List() error {
	config, err := s.Load()
	if err != nil {
		return err
	}

	fmt.Printf("Config file: %s\n\n", s.configPath)

	if len(config) == 0 {
		fmt.Println("No configuration set.")
		fmt.Println("\nRun 'orb config set <key> <value>' to configure.")
		return nil
	}

	for _, known := range KnownKeys {
		value := config[known.Name]
		if value == "" {
			value = os.Getenv(known.Name)
		}

		status := ""
		if known.Required && value == "" {
			status = " (missing)"
		} else if !known.Required && value == "" {
			status = " (optional)"
		}

		// Mask sensitive values
		displayValue := value
		if value != "" && (strings.Contains(known.Name, "TOKEN") || strings.Contains(known.Name, "SECRET")) {
			if len(value) > 8 {
				displayValue = value[:4] + "..." + value[len(value)-4:]
			} else {
				displayValue = "****"
			}
		}

		if displayValue == "" {
			displayValue = "(not set)"
		}

		fmt.Printf("%-25s %s%s\n", known.Name+":", displayValue, status)
	}

	// Print any unknown keys
	for key, value := range config {
		isKnown := false
		for _, known := range KnownKeys {
			if known.Name == key {
				isKnown = true
				break
			}
		}
		if !isKnown {
			fmt.Printf("%-25s %s\n", key+":", value)
		}
	}

	return nil
}

// Init creates a config file with empty/placeholder values
func (s *Service) Init(force bool) error {
	if err := s.EnsureConfigDir(); err != nil {
		return err
	}

	// Check if file exists
	if _, err := os.Stat(s.configPath); err == nil && !force {
		return fmt.Errorf("config file already exists at %s\nUse --force to overwrite", s.configPath)
	}

	file, err := os.Create(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	fmt.Fprintln(file, "# Orb Configuration")
	fmt.Fprintln(file, "# https://github.com/gonsis10/orb")
	fmt.Fprintln(file, "")

	for _, key := range KnownKeys {
		if key.Required {
			fmt.Fprintf(file, "# %s (required)\n", key.Description)
		} else {
			fmt.Fprintf(file, "# %s (optional)\n", key.Description)
		}
		fmt.Fprintf(file, "%s=\n\n", key.Name)
	}

	// Set restrictive permissions
	if err := os.Chmod(s.configPath, 0600); err != nil {
		fmt.Printf("Warning: could not set file permissions: %v\n", err)
	}

	fmt.Printf("Created config file: %s\n", s.configPath)
	fmt.Println("\nEdit the file to add your configuration values.")
	return nil
}

// Path prints the config file path
func (s *Service) Path() {
	fmt.Println(s.configPath)
}
