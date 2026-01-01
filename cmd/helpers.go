package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	Domain = "simoonsong.com"
	CloudflaredSvc = "cloudflared.service"
	ConfigPath = "/etc/cloudflared/config.yml"
	LockPath = "/tmp/orb-cloudflared.lock"
	LockTimeout = 30 * time.Second
)

type IngressRule struct {
	Hostname string `yaml:"hostname,omitempty"`
	Service  string `yaml:"service"`
}

type TunnelConfig struct {
	Tunnel string `yaml:"tunnel"`
	CredentialsFile string `yaml:"credentials-file"`
	Ingress []IngressRule `yaml:"ingress"`
}

func withLock(fn func() error) error {
	f, err := os.OpenFile(LockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			fmt.Println("Another orb operation is in progress, waiting for completion...")

			if err := lockWithTimeout(f, LockTimeout); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("failed to acquire lock: %w", err)
		}
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}

func lockWithTimeout(f *os.File, timeout time.Duration) error {
	done := make(chan error, 1)

	go func() {
		done <- syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for lock after %v — is another orb command stuck?", timeout)
	}
}

func loadConfig() (*TunnelConfig, error) {
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cloudflared config not found at %s", ConfigPath)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied reading %s — try with sudo", ConfigPath)
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var config TunnelConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid YAML in config: %w", err)
	}
	return &config, nil
}

func writeConfigAtomic(config *TunnelConfig) error {
	out, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(ConfigPath)
	tmp := filepath.Join(dir, ".config.yml.tmp")

	if err := os.WriteFile(tmp, out, 0644); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied writing to %s — try with sudo", dir)
		}
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	if err := os.Rename(tmp, ConfigPath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to replace config: %w", err)
	}

	return nil
}

func ensureCatchAllLast(config *TunnelConfig) error {
	if len(config.Ingress) == 0 {
		return errors.New("config has no ingress rules — add a catch-all rule first")
	}

	last := config.Ingress[len(config.Ingress)-1]
	if last.Hostname != "" {
		return fmt.Errorf("last ingress rule must be a catch-all (no hostname), got hostname=%q", last.Hostname)
	}

	return nil
}

func hostnameFor(subdomain string) string {
	return fmt.Sprintf("%s.%s", subdomain, Domain)
}

func serviceFor(port string) string {
	return fmt.Sprintf("http://localhost:%s", port)
}

func ensurePortListening(port string) error {
	addr := "127.0.0.1:" + port
	conn, err := net.DialTimeout("tcp", addr, 800*time.Millisecond)
	if err != nil {
		return fmt.Errorf("nothing listening on %s — start your service first", addr)
	}
	conn.Close()
	return nil
}

func findIngressIndex(config *TunnelConfig, hostname string) int {
	for i, rule := range config.Ingress {
		if rule.Hostname == hostname {
			return i
		}
	}
	return -1
}
