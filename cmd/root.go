package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "orb",
	Short: "CLI for managing Cloudflare Tunnels with Zero Trust access control",
	Long:  `orb exposes local services through Cloudflare Tunnel with Zero Trust access control.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(accessCmd)
	rootCmd.AddCommand(scheduleCmd)
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
