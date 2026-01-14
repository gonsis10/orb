package tunnel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Environment holds validated environment configuration
type Environment struct {
	Domain     string
	ConfigPath string
}

// LoadEnvironment loads and validates required environment variables
func LoadEnvironment() (*Environment, error) {
	domain := os.Getenv("DOMAIN")
	if domain == "" {
		return nil, fmt.Errorf("DOMAIN environment variable is required")
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		return nil, fmt.Errorf("CONFIG_PATH environment variable is required")
	}

	return &Environment{
		Domain:     domain,
		ConfigPath: configPath,
	}, nil
}

// Domain returns the configured domain (for backward compatibility)
var Domain = os.Getenv("DOMAIN")

// ConfigPath returns the configured config path (for backward compatibility)
var ConfigPath = os.Getenv("CONFIG_PATH")

// IngressRule represents a single ingress rule in the cloudflared configuration
type IngressRule struct {
	Hostname string `yaml:"hostname,omitempty"`
	Service  string `yaml:"service"`
}

// Config represents the cloudflared YAML configuration structure
type Config struct {
	Tunnel          string        `yaml:"tunnel"`
	CredentialsFile string        `yaml:"credentials-file"`
	Ingress         []IngressRule `yaml:"ingress"`
}

// ConfigManager handles loading and saving cloudflared configuration files
type ConfigManager struct {
	path string
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{path: configPath}
}

// Load reads and parses the cloudflared config from file
func (m *ConfigManager) Load() (*Config, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cloudflared config not found at %s", m.path)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied reading %s - try with sudo", m.path)
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid YAML in config: %w", err)
	}
	return &config, nil
}

// Save writes the cloudflared config to file atomically
func (m *ConfigManager) Save(config *Config) error {
	out, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(m.path)
	tmp := filepath.Join(dir, ".config.yml.tmp")

	if err := os.WriteFile(tmp, out, 0644); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied writing to %s - try with sudo", dir)
		}
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	if err := os.Rename(tmp, m.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to replace config: %w", err)
	}

	return nil
}

func (m *ConfigManager) Backup(config *Config) *Config {
	backup := *config
	backup.Ingress = make([]IngressRule, len(config.Ingress))
	copy(backup.Ingress, config.Ingress)
	return &backup
}

// ModifySubdomainPort updates the port and service type for a given subdomain in the config
func (m *ConfigManager) ModifySubdomainPort(config *Config, subdomain, port, serviceType string) error {
	hostname := HostnameFor(subdomain)
	service := ServiceURL(port, serviceType)

	idx := m.FindIngressIndex(config, hostname)
	if idx == -1 {
		return fmt.Errorf("no ingress rule found for subdomain %q", subdomain)
	}

	config.Ingress[idx].Service = service
	return nil
}

// EnsureCatchAllLast validates that the last ingress rule is a catch-all
func (m *ConfigManager) EnsureCatchAllLast(config *Config) error {
	if len(config.Ingress) == 0 {
		return errors.New("config has no ingress rules - add a catch-all rule first")
	}

	last := config.Ingress[len(config.Ingress)-1]
	if last.Hostname != "" {
		return fmt.Errorf("last ingress rule must be a catch-all (no hostname), got hostname=%q", last.Hostname)
	}

	return nil
}

// FindIngressIndex finds the index of an ingress rule by hostname
func (m *ConfigManager) FindIngressIndex(config *Config, hostname string) int {
	for i, rule := range config.Ingress {
		if rule.Hostname == hostname {
			return i
		}
	}
	return -1
}

// HostnameFor formats a full hostname from a subdomain
func HostnameFor(subdomain string) string {
	return fmt.Sprintf("%s.%s", subdomain, Domain)
}

// Service type constants
const (
	ServiceTypeHTTP  = "http"
	ServiceTypeHTTPS = "https"
	ServiceTypeTCP   = "tcp"
	ServiceTypeUDP   = "udp"
	ServiceTypeSSH   = "ssh"
	ServiceTypeRDP   = "rdp"
	ServiceTypeSMB   = "smb"
	ServiceTypeUnix  = "unix"

	DefaultServiceType = ServiceTypeHTTP
)

// ValidServiceTypes contains all supported service types
var ValidServiceTypes = []string{
	ServiceTypeHTTP,
	ServiceTypeHTTPS,
	ServiceTypeTCP,
	ServiceTypeUDP,
	ServiceTypeSSH,
	ServiceTypeRDP,
	ServiceTypeSMB,
	ServiceTypeUnix,
}

// Access level constants for Zero Trust policies
const (
	AccessLevelPublic  = "public"  // No access policy (default)
	AccessLevelPrivate = "private" // Only authenticated user
	AccessLevelGroup   = "group"   // Specific group of users

	DefaultAccessLevel = AccessLevelPublic
)

// ValidAccessLevels contains all supported access levels
var ValidAccessLevels = []string{
	AccessLevelPublic,
	AccessLevelPrivate,
	AccessLevelGroup,
}

// ServiceURL formats a service URL from a port number and service type
func ServiceURL(port, serviceType string) string {
	return fmt.Sprintf("%s://localhost:%s", serviceType, port)
}
