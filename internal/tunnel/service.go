// Package tunnel provides management for Cloudflare Tunnel ingress rules.
// It handles exposing, unexposing, updating, and listing local services
// through Cloudflare Tunnel.
package tunnel

import (
	"fmt"

	"orb/internal/dns"
)

// Service struct for tunnel operations
type Service struct {
	config     *ConfigManager
	cloudflare *dns.Client
	env        *Environment
}

// NewService creates a new tunnel service
func NewService() (*Service, error) {
	// Validate environment variables first
	env, err := LoadEnvironment()
	if err != nil {
		return nil, err
	}

	// Create Cloudflare client
	client, err := dns.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create cloudflare client: %w", err)
	}

	return &Service{
		config:     NewConfigManager(env.ConfigPath),
		cloudflare: client,
		env:        env,
	}, nil
}

// Expose makes a local port accessible through a Cloudflare Tunnel subdomain
func (s *Service) Expose(subdomain, port string) error {
	// validation of arguments and if server is running
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}
	if err := ValidatePort(port); err != nil {
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

	// restart cloudflared service
	if err := s.cloudflare.RestartCloudflaredService(cfg.Tunnel, host); err != nil {
		return fmt.Errorf("failed to restart cloudflared service: %w", err)
	}

	fmt.Printf("✔ Exposed %s → %s\n", host, svc)
	fmt.Printf("  Visit: https://%s\n", host)
	return nil
}

// Unexpose removes a subdomain from the Cloudflare Tunnel
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

	// remove domain from cloudflare dashboard
	fmt.Printf("Removing DNS route for %s...\n", host)
	if err := s.cloudflare.RemoveDNSRoute(cfg.Tunnel, host); err != nil {
		return fmt.Errorf("config updated but failed to remove DNS route: %w", err)
	}

	// restart cloudflared service
	if err := s.cloudflare.RestartCloudflaredService(cfg.Tunnel, host); err != nil {
		return fmt.Errorf("failed to restart cloudflared service: %w", err)
	}

	fmt.Printf("✔ Removed %s (was → %s)\n", host, oldService)
	return nil
}

// Update changes the port mapping for an existing subdomain
func (s *Service) Update(subdomain, port string) error {
	// validate arguments
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}
	if err := ValidatePort(port); err != nil {
		return err
	}

	// load cloudflare config
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}

	// modify subdomain port in config
	if err := s.config.ModifySubdomainPort(cfg, subdomain, port); err != nil {
		return err
	}

	// save to yaml
	if err := s.config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("✔ Updated %s to point to %s\n", HostnameFor(subdomain), ServiceFor(port))
	return nil
}

// List displays all exposed subdomains and their port mappings
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
