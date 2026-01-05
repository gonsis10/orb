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
  deploy    Deploy dockerized services`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(tunnelCmd)
}
