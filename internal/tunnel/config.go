package tunnel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	Domain     = os.Getenv("DOMAIN")
	ConfigPath = os.Getenv("CONFIG_PATH")
)

type IngressRule struct {
	Hostname string `yaml:"hostname,omitempty"`
	Service string `yaml:"service"`
}

type Config struct {
	Tunnel string `yaml:"tunnel"`
	CredentialsFile string `yaml:"credentials-file"`
	Ingress []IngressRule `yaml:"ingress"`
}

type ConfigManager struct {
	path string
}

func NewConfigManager() *ConfigManager {
	return &ConfigManager{path: ConfigPath}
}

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

func (m *ConfigManager) FindIngressIndex(config *Config, hostname string) int {
	for i, rule := range config.Ingress {
		if rule.Hostname == hostname {
			return i
		}
	}
	return -1
}

func HostnameFor(subdomain string) string {
	return fmt.Sprintf("%s.%s", subdomain, Domain)
}

func ServiceFor(port string) string {
	return fmt.Sprintf("http://localhost:%s", port)
}
