package cmd

import (
	"fmt"
	"strings"

	"orb/internal/tunnel"

	"github.com/spf13/cobra"
)

var (
	tunnelSvc    *tunnel.Service
	exposeType   string
	exposeAccess string
	exposeEmails string
	updateType   string
	logsFollow   bool
	logsLines    int
	serviceDesc  = fmt.Sprintf("Service type: %s", strings.Join(tunnel.ValidServiceTypes, ", "))
	accessDesc   = fmt.Sprintf("Access level: %s", strings.Join(tunnel.ValidAccessLevels, ", "))
)

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
	tunnelCmd.AddCommand(healthCmd)
	tunnelCmd.AddCommand(restartCmd)
	tunnelCmd.AddCommand(statusCmd)
	tunnelCmd.AddCommand(logsCmd)

	exposeCmd.Flags().StringVarP(&exposeType, "type", "t", tunnel.DefaultServiceType, serviceDesc)
	exposeCmd.Flags().StringVarP(&exposeAccess, "access", "a", tunnel.DefaultAccessLevel, accessDesc)
	exposeCmd.Flags().StringVar(&exposeEmails, "emails", "", "Comma-separated emails for group access (only with --access=group)")
	updateCmd.Flags().StringVarP(&updateType, "type", "t", tunnel.DefaultServiceType, serviceDesc)
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow logs in real-time")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 50, "Number of lines to show")
}

var exposeCmd = &cobra.Command{
	Use:   "expose <subdomain> <port>",
	Short: "Expose a local port at subdomain." + tunnel.Domain,
	Example: `  orb tunnel expose api 8080
  orb tunnel expose api 8080 --type tcp
  orb tunnel expose api 8080 --access private
  orb tunnel expose api 8080 --access group --emails user@example.com,friend@example.com`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnelSvc.Expose(args[0], args[1], exposeType, exposeAccess, exposeEmails)
	},
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
	Use:     "update <subdomain> <port>",
	Short:   "Update the port for an exposed subdomain.",
	Example: "  orb tunnel update api 9090\n  orb tunnel update api 9090 --type tcp",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnelSvc.Update(args[0], args[1], updateType)
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

var healthCmd = &cobra.Command{
	Use:                   "health <subdomain>",
	Short:                 "Check if a subdomain is healthy and reachable",
	Example:               "  orb tunnel health api",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnelSvc.Health(args[0])
	},
}

var restartCmd = &cobra.Command{
	Use:                   "restart",
	Short:                 "Restart the cloudflared service",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnelSvc.Restart()
	},
}

var statusCmd = &cobra.Command{
	Use:                   "status",
	Short:                 "Show the cloudflared service status",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tunnelSvc.Status()
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs [subdomain]",
	Short: "Show cloudflared service logs",
	Long: `Show cloudflared service logs, optionally filtered by subdomain.

Examples:
  orb tunnel logs              # Show last 50 log lines
  orb tunnel logs api          # Show logs for api subdomain
  orb tunnel logs api -f       # Follow logs for api subdomain in real-time
  orb tunnel logs -n 100       # Show last 100 log lines`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		subdomain := ""
		if len(args) > 0 {
			subdomain = args[0]
		}
		return tunnelSvc.Logs(subdomain, logsLines, logsFollow)
	},
}
