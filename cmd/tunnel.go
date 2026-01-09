package cmd

import (
	"orb/internal/tunnel"

	"github.com/spf13/cobra"
)

var tunnelSvc *tunnel.Service
var exposeType string

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Manage Cloudflare Tunnel ingress rules",
	Long: `Expose local services through Cloudflare Tunnel.

Examples:
  orb tunnel expose api 8080    # Expose localhost:8080 at api.simoonsong.com
  orb tunnel unexpose api       # Remove the api subdomain
  orb tunnel update api 9090    # Update api subdomain to point to localhost:9090
  orb tunnel list               # Show all exposed services`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		tunnelSvc, err = tunnel.NewService()
		return err
	},
}

func init() {
	tunnelCmd.AddCommand(exposeCmd)
	tunnelCmd.AddCommand(unexposeCmd)
	tunnelCmd.AddCommand(updateCmd)
	tunnelCmd.AddCommand(listCmd)
}

var exposeCmd = &cobra.Command{
	Use:     "expose <subdomain> <port>",
	Short:   "Expose a local port at subdomain." + tunnel.Domain,
	Example: "  orb tunnel expose api 8080\n  orb tunnel expose api 8080 --type tcp",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnelSvc.Expose(args[0], args[1], exposeType)
	},
}

func init() {
	exposeCmd.Flags().StringVarP(&exposeType, "type", "t", "http", "Service type: http, tcp, or rtcp")
}

var unexposeCmd = &cobra.Command{
	Use:                   "unexpose <subdomain>",
	Short:                 "Remove an exposed subdomain.",
	Example:               "  orb tunnel unexpose api",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnelSvc.Unexpose(args[0])
	},
}

var updateCmd = &cobra.Command{
	Use:                   "update <subdomain> <port>",
	Short:                 "Update the port for an exposed subdomain.",
	Example:               "  orb tunnel update api 9090",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnelSvc.Update(args[0], args[1])
	},
}

var listCmd = &cobra.Command{
	Use:                   "list",
	Short:                 "List all exposed subdomains",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnelSvc.List()
	},
}
