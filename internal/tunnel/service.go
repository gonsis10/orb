package tunnel

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"orb/internal/dns"

	"github.com/olekukonko/tablewriter"
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
func (s *Service) Expose(subdomain, port, serviceType string) error {
	// validation of arguments and if server is running
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}
	if err := ValidatePort(port); err != nil {
		return err
	}
	if err := ValidateServiceType(serviceType); err != nil {
		return err
	}

	// get hostname and service
	host := HostnameFor(subdomain)
	svc := ServiceURL(port, serviceType)

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

	// start of TRANSACTION
	orginalCfg := s.config.Backup(cfg)

	// combine catchall and new subdomain to form new cloudlfare yaml
	catchAll := cfg.Ingress[len(cfg.Ingress)-1]

	configSaved := false
	dnsAdded := false

	defer func() {
		if !dnsAdded {
			return
		}

		// rollback and remove dns route
		fmt.Printf("Rolling back: Removing DNS route for %s...\n", host)
		if err := s.cloudflare.RemoveDNSRoute(orginalCfg.Tunnel, host); err != nil {
			fmt.Printf("Failed to rollback DNS route for %s: %v\n", host, err)
		}

		if configSaved {
			fmt.Println("Rolling back: Restoring original config...")
			if err := s.config.Save(orginalCfg); err != nil {
				fmt.Printf("Failed to restore original config: %v\n", err)
			}
		}
	}()

	cfg.Ingress = append(cfg.Ingress[:len(cfg.Ingress)-1], IngressRule{Hostname: host, Service: svc}, catchAll)

	// save to yaml file
	if err := s.config.Save(cfg); err != nil {
		return err
	}
	configSaved = true

	// create dns route
	fmt.Printf("Creating DNS route for %s...\n", host)
	if err := s.cloudflare.CreateDNSRoute(cfg.Tunnel, host); err != nil {
		return fmt.Errorf("config updated but failed to create DNS route: %w", err)
	}
	dnsAdded = true

	// restart cloudflared service
	if err := s.cloudflare.RestartCloudflaredService(cfg.Tunnel, host); err != nil {
		return fmt.Errorf("failed to restart cloudflared service: %w", err)
	}

	// reset rollback
	configSaved = false
	dnsAdded = false

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

	// start of TRANSACTION
	orginalCfg := s.config.Backup(cfg)
	oldService := cfg.Ingress[idx].Service

	configSaved := false
	dnsRemoved := false

	defer func() {
		if !dnsRemoved {
			return
		}

		// rollback and re create dns route
		fmt.Printf("Rolling back: Re-adding DNS route for %s...\n", host)
		if err := s.cloudflare.CreateDNSRoute(orginalCfg.Tunnel, host); err != nil {
			fmt.Printf("Failed to rollback DNS route for %s: %v\n", host, err)
		}

		if configSaved {
			fmt.Println("Rolling back: Restoring original config...")
			if err := s.config.Save(orginalCfg); err != nil {
				fmt.Printf("Failed to restore original config: %v\n", err)
			}
		}
	}()

	// save new yaml without previous ingress rule
	cfg.Ingress = append(cfg.Ingress[:idx], cfg.Ingress[idx+1:]...)

	// save to yaml
	if err := s.config.Save(cfg); err != nil {
		return err
	}
	configSaved = true

	// remove domain from cloudflare dashboard
	fmt.Printf("Removing DNS route for %s...\n", host)
	if err := s.cloudflare.RemoveDNSRoute(cfg.Tunnel, host); err != nil {
		return fmt.Errorf("config updated but failed to remove DNS route: %w", err)
	}
	dnsRemoved = true

	// restart cloudflared service
	if err := s.cloudflare.RestartCloudflaredService(cfg.Tunnel, host); err != nil {
		return fmt.Errorf("failed to restart cloudflared service: %w", err)
	}

	// disable rollback
	dnsRemoved = false
	configSaved = false

	fmt.Printf("✔ Removed %s (was → %s)\n", host, oldService)
	return nil
}

// Update changes the port mapping for an existing subdomain
func (s *Service) Update(subdomain, port, serviceType string) error {
	// validate arguments
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}
	if err := ValidatePort(port); err != nil {
		return err
	}
	if err := ValidateServiceType(serviceType); err != nil {
		return err
	}

	// load cloudflare config
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}

	// start of TRANSACTION
	orginalCfg := s.config.Backup(cfg)

	configSaved := false

	defer func() {
		if !configSaved {
			return
		}

		fmt.Println("Rolling back: Restoring original config...")
		if err := s.config.Save(orginalCfg); err != nil {
			fmt.Printf("Failed to restore original config: %v\n", err)
		}
	}()

	// modify subdomain port in config
	if err := s.config.ModifySubdomainPort(cfg, subdomain, port, serviceType); err != nil {
		return err
	}

	// save to yaml
	if err := s.config.Save(cfg); err != nil {
		return err
	}
	configSaved = true

	// restart cloudflared service
	if err := s.cloudflare.RestartCloudflaredService(cfg.Tunnel, HostnameFor(subdomain)); err != nil {
		return fmt.Errorf("failed to restart cloudflared service: %w", err)
	}

	// reset rollback
	configSaved = false

	fmt.Printf("✔ Updated %s to point to %s\n", HostnameFor(subdomain), ServiceURL(port, serviceType))
	return nil
}

// Health checks if a subdomain is healthy and reachable
func (s *Service) Health(subdomain string) error {
	// validate subdomain
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}

	// get hostname for subdomain
	host := fmt.Sprintf("%s.%s", subdomain, s.env.Domain)

	// load cloudflare config
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}

	// check if subdomain exists in config
	idx := s.config.FindIngressIndex(cfg, host)
	if idx == -1 {
		return fmt.Errorf("✖ %s is not currently exposed", host)
	}

	url := fmt.Sprintf("https://%s", host)
	fmt.Printf("Checking health of %s...\n", url)

	// create http client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	// make request
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("✖ %s is unhealthy\n", host)
		fmt.Printf("  Error: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	// check status code
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		fmt.Printf("✔ %s is healthy (status: %d %s)\n", host, resp.StatusCode, http.StatusText(resp.StatusCode))
	} else {
		fmt.Printf("⚠ %s returned status %d %s\n", host, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return nil
}

// checkHealth makes an HTTP request to check if a hostname is healthy
func (s *Service) checkHealth(hostname string) string {
	url := fmt.Sprintf("https://%s", hostname)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return "✖ unhealthy"
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return "✔ healthy"
	}
	return fmt.Sprintf("⚠ %d", resp.StatusCode)
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

	// create table
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("URL", "Target", "Status")

	fmt.Println("\nChecking health of exposed services...")

	// add rows to table
	for _, rule := range cfg.Ingress {
		if rule.Hostname == "" {
			continue
		}
		status := s.checkHealth(rule.Hostname)
		if err := table.Append(
			fmt.Sprintf("https://%s", rule.Hostname),
			rule.Service,
			status,
		); err != nil {
			return fmt.Errorf("failed to add table row: %w", err)
		}
	}

	// render table
	fmt.Println("\nExposed services:")
	if err := table.Render(); err != nil {
		return fmt.Errorf("failed to render table: %w", err)
	}

	return nil
}
