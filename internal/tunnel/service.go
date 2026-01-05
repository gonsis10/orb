package tunnel

import (
	"fmt"

	"orb/internal/cloudflared"
)

type Service struct {
	config *ConfigManager
	cloudflare *cloudflared.Client
}

func NewService() *Service {
	return &Service{
		config:     NewConfigManager(),
		cloudflare: cloudflared.New(),
	}
}

func (s *Service) Expose(subdomain, port string) error {
	// validation of arguments and if server is running
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}
	if err := ValidatePort(port); err != nil {
		return err
	}
	if err := EnsurePortListening(port); err != nil {
		return err
	}

	// get hostname and service
	host := HostnameFor(subdomain)
	svc := ServiceFor(port)

	// get cloudflare config yaml
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}

	// ensures if there exists ingress and last ingress is catch all
	if err := s.config.EnsureCatchAllLast(cfg); err != nil {
		return err
	}

	// checks if hostname already exists in the ingress
	if idx := s.config.FindIngressIndex(cfg, host); idx != -1 {
		existing := cfg.Ingress[idx].Service
		if existing == svc {
			fmt.Printf("ℹ️  %s already points to %s (no changes needed)\n", host, svc)
			return nil
		}
		return fmt.Errorf("✖ %s is already mapped to %s\n  Run `orb tunnel unexpose %s` first, or use a different subdomain", host, existing, subdomain)
	}

	// combine catchall and new subdomain to form new cloudlfare yaml
	catchAll := cfg.Ingress[len(cfg.Ingress)-1]
	cfg.Ingress = append(cfg.Ingress[:len(cfg.Ingress)-1], IngressRule{Hostname: host, Service: svc}, catchAll)

	// save to yaml file
	if err := s.config.Save(cfg); err != nil {
		return err
	}

	// create dns route
	fmt.Printf("Creating DNS route for %s...\n", host)
	if err := s.cloudflare.CreateDNSRoute(cfg.Tunnel, host); err != nil {
		return fmt.Errorf("config updated but failed to create DNS route: %w", err)
	}
	
	fmt.Printf("✔ Exposed %s → %s\n", host, svc)
	fmt.Printf("  Visit: https://%s\n", host)
	return nil
}

func (s *Service) Unexpose(subdomain string) error {
	// validate subdomain
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}

	// get hostname for subdomain
	host := HostnameFor(subdomain)

	// load cloudflare config
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}

	// get ingress index for hostname
	idx := s.config.FindIngressIndex(cfg, host)
	if idx == -1 {
		return fmt.Errorf("✖ %s is not currently exposed", host)
	}

	// get old service
	oldService := cfg.Ingress[idx].Service
	// save new yaml without previous ingress rule
	cfg.Ingress = append(cfg.Ingress[:idx], cfg.Ingress[idx+1:]...)

	// save to yaml
	if err := s.config.Save(cfg); err != nil {
		return err
	}

	// TODO: add remove func for domain from cloudflare dashboard

	fmt.Printf("✔ Removed %s (was → %s)\n", host, oldService)
	return nil
}

func (s *Service) List() error {
	// load cloudflare config
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}

	// check if ingress rule is less than or equal to 1
	if len(cfg.Ingress) <= 1 {
		fmt.Println("No services exposed (only catch-all rule present)")
		return nil
	}

	// print list of exposed services
	fmt.Println("Exposed services:")
	for _, rule := range cfg.Ingress {
		if rule.Hostname == "" {
			continue
		}
		fmt.Printf("  https://%-30s → %s\n", rule.Hostname, rule.Service)
	}

	return nil
}
