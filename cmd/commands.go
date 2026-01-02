package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/spf13/cobra"
)

var (
	subdomainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	portRe = regexp.MustCompile(`^\d{1,5}$`)
)

var rootCmd = &cobra.Command{
	Use:   "orb",
	Short: "Manage Cloudflare Tunnel ingress rules",
	Long: `orb is a CLI for exposing local services through Cloudflare Tunnel.

Examples:
  orb expose api 8080      # Expose localhost:8080 at api.simoonsong.com
  orb unexpose api         # Remove the api subdomain
  orb list                 # Show all exposed services`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(exposeCmd)
	rootCmd.AddCommand(unexposeCmd)
	rootCmd.AddCommand(listCmd)
}

var exposeCmd = &cobra.Command{
	Use:                   "expose <subdomain> <port>",
	Short:                 "Expose a local port at subdomain." + Domain,
	Example:               "  orb expose api 8080",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	RunE:                  runExpose,
}

func runExpose(cmd *cobra.Command, args []string) error {
	subdomain, port := args[0], args[1]

	if err := validateSubdomain(subdomain); err != nil {
		return err
	}
	if err := validatePort(port); err != nil {
		return err
	}
	if err := ensurePortListening(port); err != nil {
		return err
	}

	host := hostnameFor(subdomain)
	svc := serviceFor(port)

	return withLock(func() error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if err := ensureCatchAllLast(cfg); err != nil {
			return err
		}

		// check if hostname already exists
		if idx := findIngressIndex(cfg, host); idx != -1 {
			existing := cfg.Ingress[idx].Service
			if existing == svc {
				fmt.Printf("ℹ️  %s already points to %s (no changes needed)\n", host, svc)
				return nil
			}
			return fmt.Errorf("✖ %s is already mapped to %s\n  Run `orb unexpose %s` first, or use a different subdomain", host, existing, subdomain)
		}

		// insert new rule before the catch-all (last element)
		catchAll := cfg.Ingress[len(cfg.Ingress)-1]
		cfg.Ingress = append(cfg.Ingress[:len(cfg.Ingress)-1], IngressRule{Hostname: host, Service: svc}, catchAll)

		if err := writeConfigAtomic(cfg); err != nil {
			return err
		}

		if err := reloadCloudflared(); err != nil {
			return fmt.Errorf("config updated but failed to reload cloudflared: %w", err)
		}

		fmt.Printf("✔ Exposed %s → %s\n", host, svc)
		fmt.Printf("  Visit: https://%s\n", host)
		return nil
	})
}

var unexposeCmd = &cobra.Command{
	Use:                   "unexpose <subdomain>",
	Short:                 "Remove a subdomain from the tunnel",
	Example:               "  orb unexpose api",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE:                  runUnexpose,
}

func runUnexpose(cmd *cobra.Command, args []string) error {
	subdomain := args[0]

	if err := validateSubdomain(subdomain); err != nil {
		return err
	}

	host := hostnameFor(subdomain)

	return withLock(func() error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		idx := findIngressIndex(cfg, host)
		if idx == -1 {
			return fmt.Errorf("✖ %s is not currently exposed", host)
		}

		oldService := cfg.Ingress[idx].Service

		// remove the rule
		cfg.Ingress = append(cfg.Ingress[:idx], cfg.Ingress[idx+1:]...)

		if err := writeConfigAtomic(cfg); err != nil {
			return err
		}

		if err := reloadCloudflared(); err != nil {
			return fmt.Errorf("config updated but failed to reload cloudflared: %w", err)
		}

		fmt.Printf("✔ Removed %s (was → %s)\n", host, oldService)
		return nil
	})
}

var listCmd = &cobra.Command{
	Use:                   "list",
	Short:                 "List all exposed services",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE:                  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
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
			continue // skip catch-all
		}
		fmt.Printf("  https://%-30s → %s\n", rule.Hostname, rule.Service)
	}

	return nil
}

func validateSubdomain(s string) error {
	if !subdomainRe.MatchString(s) {
		return errors.New("invalid subdomain: use lowercase letters, digits, and hyphens (must start/end with alphanumeric)")
	}
	return nil
}

func validatePort(p string) error {
	if !portRe.MatchString(p) {
		return errors.New("invalid port: must be a number between 1-65535")
	}
	return nil
}

func reloadCloudflared() error {
	// try reload first (graceful), fall back to restart
	if err := exec.Command("sudo", "systemctl", "reload", CloudflaredSvc).Run(); err == nil {
		return nil
	}

	if err := exec.Command("sudo", "systemctl", "restart", CloudflaredSvc).Run(); err != nil {
		return fmt.Errorf("failed to reload/restart %s: %w", CloudflaredSvc, err)
	}

	return nil
}
