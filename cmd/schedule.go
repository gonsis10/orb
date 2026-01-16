package cmd

import (
	"orb/internal/scheduler"

	"github.com/spf13/cobra"
)

var schedulerSvc *scheduler.Service

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage scheduled tasks using cron",
	Example: `  orb schedule add backup "0 2 * * *" "./backup.sh"     # Daily at 2am
  orb schedule add hourly-sync "0 * * * *" "sync.py"    # Every hour
  orb schedule list                                      # Show all schedules
  orb schedule remove backup                             # Remove a schedule`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		schedulerSvc, err = scheduler.NewService()
		return err
	},
}

func init() {
	scheduleCmd.AddCommand(scheduleAddCmd)
	scheduleCmd.AddCommand(scheduleRemoveCmd)
	scheduleCmd.AddCommand(scheduleListCmd)
}

var scheduleAddCmd = &cobra.Command{
	Use:   "add <name> <cron> <command>",
	Short: "Add a new scheduled task",
	Long: `Add a new scheduled task that runs on a cron schedule.

Cron format: minute hour day month weekday
  - minute:  0-59
  - hour:    0-23
  - day:     1-31
  - month:   1-12
  - weekday: 0-7 (0 and 7 are Sunday)

Use * for "every" and */N for "every N"`,
	Example: `  orb schedule add backup "0 2 * * *" "./scripts/backup.sh"   # Daily at 2:00 AM
  orb schedule add sync "*/30 * * * *" "python sync.py"        # Every 30 minutes
  orb schedule add weekly "0 9 * * 1" "/usr/local/bin/report"  # Mondays at 9 AM`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		return schedulerSvc.Add(args[0], args[1], args[2])
	},
}

var scheduleRemoveCmd = &cobra.Command{
	Use:                   "remove <name>",
	Short:                 "Remove a scheduled task",
	Example:               "  orb schedule remove backup",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return schedulerSvc.Remove(args[0])
	},
}

var scheduleListCmd = &cobra.Command{
	Use:                   "list",
	Short:                 "List all scheduled tasks",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return schedulerSvc.List()
	},
}
