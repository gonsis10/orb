package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "orb",
	Short: "CLI for managing Cloudflare Tunnel and deployments",
	Long: `orb is a CLI for managing infrastructure services.

Commands are grouped by function:
  tunnel    Manage Cloudflare Tunnel ingress rules
  deploy    Deploy services (coming soon)

Examples:
  orb tunnel expose api 8080    # Expose localhost:8080 at api.simoonsong.com
  orb tunnel unexpose api       # Remove the api subdomain
  orb tunnel list               # Show all exposed services`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(tunnelCmd)
}
