package cmd

import (
	"os"

	"orb/internal/doctor"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose common issues with orb configuration",
	Long: `Run diagnostic checks to identify common issues with orb setup.

Checks performed:
  - Environment variables (DOMAIN, CONFIG_PATH, CLOUDFLARE_*)
  - Config file existence and readability
  - cloudflared binary installation
  - cloudflared service status
  - Cloudflare API token validity
  - Zone and account access permissions
  - Internet connectivity
  - DNS resolution`,
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		svc := doctor.NewService()
		svc.RunAll()
		svc.PrintResults()

		if svc.HasFailures() {
			os.Exit(1)
		}
	},
}
