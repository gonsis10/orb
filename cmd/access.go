package cmd

import (
	"fmt"
	"os"
	"strings"

	"orb/internal/tunnel"

	"github.com/spf13/cobra"
)

var (
	accessSvc *tunnel.Service
)

var accessCmd = &cobra.Command{
	Use:   "access",
	Short: "Manage Cloudflare Access groups",
	Example: `  orb access create friends user1@example.com,user2@example.com
  orb access list
  orb access delete friends`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		accessSvc, err = tunnel.NewService()
		return err
	},
}

func init() {
	updateGroupCmd.Flags().StringP("add", "a", "", "Comma-separated emails to add")
	updateGroupCmd.Flags().StringP("remove", "r", "", "Comma-separated emails to remove")

	accessCmd.AddCommand(createGroupCmd)
	accessCmd.AddCommand(listGroupsCmd)
	accessCmd.AddCommand(deleteGroupCmd)
	accessCmd.AddCommand(updateGroupCmd)
	accessCmd.AddCommand(showGroupCmd)
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

var updateGroupCmd = &cobra.Command{
	Use:   "update <group-name>",
	Short: "Add or remove members from an Access group",
	Example: `  orb access update friends --add user3@example.com
  orb access update friends --remove user1@example.com
  orb access update friends -a new@example.com -r old@example.com`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addFlag, _ := cmd.Flags().GetString("add")
		removeFlag, _ := cmd.Flags().GetString("remove")

		if addFlag == "" && removeFlag == "" {
			fmt.Fprintln(os.Stderr, "Error: must specify --add or --remove (or both)")
			os.Exit(1)
		}

		var addEmails, removeEmails []string
		if addFlag != "" {
			for _, e := range strings.Split(addFlag, ",") {
				if email := strings.TrimSpace(e); email != "" {
					addEmails = append(addEmails, email)
				}
			}
		}
		if removeFlag != "" {
			for _, e := range strings.Split(removeFlag, ",") {
				if email := strings.TrimSpace(e); email != "" {
					removeEmails = append(removeEmails, email)
				}
			}
		}

		return accessSvc.UpdateAccessGroupMembers(args[0], addEmails, removeEmails)
	},
}

var showGroupCmd = &cobra.Command{
	Use:                   "show <group-name>",
	Short:                 "Show members of an Access group",
	Example:               "  orb access show friends",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		members, err := accessSvc.GetAccessGroupMembers(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Members of %q (%d):\n", args[0], len(members))
		for _, email := range members {
			fmt.Printf("  • %s\n", email)
		}
		return nil
	},
}
