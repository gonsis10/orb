package tunnel

import (
	"fmt"

	"orb/internal/cloudflared"
)

type Service struct {
	config     *ConfigManager
	cloudflare *cloudflared.Client
}

func NewService() *Service {
	return &Service{
		config:     NewConfigManager(),
		cloudflare: cloudflared.New(),
	}
}

func (s *Service) Expose(subdomain, port string) error {
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}
	if err := ValidatePort(port); err != nil {
		return err
	}
	if err := EnsurePortListening(port); err != nil {
		return err
	}

	host := HostnameFor(subdomain)
	svc := ServiceFor(port)

	cfg, err := s.config.Load()
	if err != nil {
		return err
	}

	if err := s.config.EnsureCatchAllLast(cfg); err != nil {
		return err
	}

	if idx := s.config.FindIngressIndex(cfg, host); idx != -1 {
		existing := cfg.Ingress[idx].Service
		if existing == svc {
			fmt.Printf("ℹ️  %s already points to %s (no changes needed)\n", host, svc)
			return nil
		}
		return fmt.Errorf("✖ %s is already mapped to %s\n  Run `orb tunnel unexpose %s` first, or use a different subdomain", host, existing, subdomain)
	}

	catchAll := cfg.Ingress[len(cfg.Ingress)-1]
	cfg.Ingress = append(cfg.Ingress[:len(cfg.Ingress)-1], IngressRule{Hostname: host, Service: svc}, catchAll)

	if err := s.config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("Creating DNS route for %s...\n", host)
	if err := s.cloudflare.CreateDNSRoute(cfg.Tunnel, host); err != nil {
		return fmt.Errorf("config updated but failed to create DNS route: %w", err)
	}
	
	fmt.Printf("✔ Exposed %s → %s\n", host, svc)
	fmt.Printf("  Visit: https://%s\n", host)
	return nil
}

func (s *Service) Unexpose(subdomain string) error {
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}

	host := HostnameFor(subdomain)

	cfg, err := s.config.Load()
	if err != nil {
		return err
	}

	idx := s.config.FindIngressIndex(cfg, host)
	if idx == -1 {
		return fmt.Errorf("✖ %s is not currently exposed", host)
	}

	oldService := cfg.Ingress[idx].Service
	cfg.Ingress = append(cfg.Ingress[:idx], cfg.Ingress[idx+1:]...)

	if err := s.config.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("✔ Removed %s (was → %s)\n", host, oldService)
	return nil
}

func (s *Service) List() error {
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Ingress) <= 1 {
		fmt.Println("No services exposed (only catch-all rule present)")
		return nil
	}

	fmt.Println("Exposed services:")
	for _, rule := range cfg.Ingress {
		if rule.Hostname == "" {
			continue
		}
		fmt.Printf("  https://%-30s → %s\n", rule.Hostname, rule.Service)
	}

	return nil
}
