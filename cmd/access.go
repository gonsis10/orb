package cmd

import (
	"orb/internal/tunnel"

	"github.com/spf13/cobra"
)

var (
	accessSvc    *tunnel.Service
	groupEmails  string
	groupInclude string
)

var accessCmd = &cobra.Command{
	Use:   "access",
	Short: "Manage Cloudflare Access groups",
	Long: `Manage Cloudflare Zero Trust Access groups for authentication.

Examples:
  orb access create friends user1@example.com,user2@example.com
  orb access list
  orb access delete friends`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		accessSvc, err = tunnel.NewService()
		return err
	},
}

func init() {
	accessCmd.AddCommand(createGroupCmd)
	accessCmd.AddCommand(listGroupsCmd)
	accessCmd.AddCommand(deleteGroupCmd)
}

var createGroupCmd = &cobra.Command{
	Use:   "create <group-name> <emails>",
	Short: "Create an Access group with email addresses",
	Example: `  orb access create friends user1@example.com,user2@example.com
  orb access create hackathon2025 alice@example.com,bob@example.com,charlie@example.com`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return accessSvc.CreateAccessGroup(args[0], args[1])
	},
}

var listGroupsCmd = &cobra.Command{
	Use:                   "list",
	Short:                 "List all Access groups",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return accessSvc.ListAccessGroups()
	},
}

var deleteGroupCmd = &cobra.Command{
	Use:                   "delete <group-name>",
	Short:                 "Delete an Access group",
	Example:               "  orb access delete friends",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return accessSvc.DeleteAccessGroup(args[0])
	},
}
