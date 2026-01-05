package cmd

import (
	"orb/internal/tunnel"

	"github.com/spf13/cobra"
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Manage Cloudflare Tunnel ingress rules",
	Long: `Expose local services through Cloudflare Tunnel.

Examples:
  orb tunnel expose api 8080    # Expose localhost:8080 at api.simoonsong.com
  orb tunnel unexpose api       # Remove the api subdomain
  orb tunnel list               # Show all exposed services`,
}

func init() {
	tunnelCmd.AddCommand(exposeCmd)
	tunnelCmd.AddCommand(unexposeCmd)
	tunnelCmd.AddCommand(listCmd)
}

var exposeCmd = &cobra.Command{
	Use:                   "expose <subdomain> <port>",
	Short:                 "Expose a local port at subdomain." + tunnel.Domain,
	Example:               "  orb tunnel expose api 8080",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnel.NewService().Expose(args[0], args[1])
	},
}

var unexposeCmd = &cobra.Command{
	Use:                   "unexpose <subdomain>",
	Short:                 "Remove an exposed subdomain.",
	Example:               "  orb tunnel unexpose api",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnel.NewService().Unexpose(args[0])
	},
}

var listCmd = &cobra.Command{
	Use:                   "list",
	Short:                 "List all exposed subdomains",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnel.NewService().List()
	},
}
