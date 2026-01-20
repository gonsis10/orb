package cmd

import (
	"fmt"

	"orb/internal/database"
	"orb/internal/tunnel"

	"github.com/spf13/cobra"
)

var dbSvc *tunnel.Service
var dbMgr *database.Service

// Database types with their default ports (for expose command)
var dbDefaults = map[string]struct {
	port        string
	serviceType string
	description string
}{
	"postgres":   {port: "5432", serviceType: "tcp", description: "PostgreSQL"},
	"mysql":      {port: "3306", serviceType: "tcp", description: "MySQL/MariaDB"},
	"redis":      {port: "6379", serviceType: "tcp", description: "Redis"},
	"mongodb":    {port: "27017", serviceType: "tcp", description: "MongoDB"},
	"memcached":  {port: "11211", serviceType: "tcp", description: "Memcached"},
	"mssql":      {port: "1433", serviceType: "tcp", description: "Microsoft SQL Server"},
	"clickhouse": {port: "9000", serviceType: "tcp", description: "ClickHouse"},
	"cassandra":  {port: "9042", serviceType: "tcp", description: "Cassandra"},
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Manage and expose databases",
	Long: `Create, manage, and expose databases through Cloudflare Tunnel.

Commands for managing database containers:
  create  - Create a new database container (Docker)
  list    - List managed databases
  start   - Start a stopped database
  stop    - Stop a running database
  delete  - Delete a database
  logs    - View database logs

Commands for exposing databases:
  expose  - Expose a database through Cloudflare Tunnel
  types   - List supported database types`,
	Example: `  orb db create postgres mydb
  orb db list
  orb db expose postgres mydb`,
}

var dbExposeCmd = &cobra.Command{
	Use:   "expose <db-type> <subdomain>",
	Short: "Expose a database through Cloudflare Tunnel",
	Long: `Expose a local database securely through Cloudflare Tunnel.

Supported database types:
  postgres   - PostgreSQL (default port: 5432)
  mysql      - MySQL/MariaDB (default port: 3306)
  redis      - Redis (default port: 6379)
  mongodb    - MongoDB (default port: 27017)
  memcached  - Memcached (default port: 11211)
  mssql      - Microsoft SQL Server (default port: 1433)
  clickhouse - ClickHouse (default port: 9000)
  cassandra  - Cassandra (default port: 9042)

Databases are exposed with TCP service type and private access by default.`,
	Example: `  orb db expose postgres mydb
  orb db expose postgres mydb --port 5433
  orb db expose mysql mydb --access team
  orb db expose redis cache --access team --expires 24h`,
	Args: cobra.ExactArgs(2),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		dbSvc, err = tunnel.NewService()
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dbType := args[0]
		subdomain := args[1]

		defaults, ok := dbDefaults[dbType]
		if !ok {
			return fmt.Errorf("unknown database type %q\nSupported: postgres, mysql, redis, mongodb, memcached, mssql, clickhouse, cassandra", dbType)
		}

		port, _ := cmd.Flags().GetString("port")
		if port == "" {
			port = defaults.port
		}

		access, _ := cmd.Flags().GetString("access")
		if access == "" {
			access = "private"
		}

		expires, _ := cmd.Flags().GetString("expires")

		fmt.Printf("Exposing %s database...\n", defaults.description)
		return dbSvc.Expose(subdomain, port, defaults.serviceType, access, expires)
	},
}

var dbTypesCmd = &cobra.Command{
	Use:                   "types",
	Short:                 "List supported database types",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Supported database types:")
		fmt.Println()
		fmt.Printf("  %-12s %-25s %s\n", "TYPE", "DESCRIPTION", "DEFAULT PORT")
		fmt.Printf("  %-12s %-25s %s\n", "----", "-----------", "------------")
		for dbType, info := range dbDefaults {
			fmt.Printf("  %-12s %-25s %s\n", dbType, info.description, info.port)
		}
	},
}

var dbCreateCmd = &cobra.Command{
	Use:   "create <db-type> <name>",
	Short: "Create a new database container",
	Long: `Create a new database container using Docker.

Supported database types: postgres, mysql, redis, mongodb

The database will be created with:
  - Data persisted to ~/.local/share/orb/databases/<name>
  - Bound to localhost only (127.0.0.1)
  - Auto-restart enabled`,
	Example: `  orb db create postgres mydb
  orb db create postgres mydb --port 5433
  orb db create mysql app-db
  orb db create redis cache`,
	Args: cobra.ExactArgs(2),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		dbMgr, err = database.NewService()
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dbType := args[0]
		name := args[1]
		port, _ := cmd.Flags().GetString("port")
		return dbMgr.Create(dbType, name, port)
	},
}

var dbListManagedCmd = &cobra.Command{
	Use:                   "list",
	Aliases:               []string{"ls"},
	Short:                 "List managed databases",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		dbMgr, err = database.NewService()
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return dbMgr.List()
	},
}

var dbStartCmd = &cobra.Command{
	Use:                   "start <name>",
	Short:                 "Start a stopped database",
	Example:               "  orb db start mydb",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		dbMgr, err = database.NewService()
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return dbMgr.Start(args[0])
	},
}

var dbStopCmd = &cobra.Command{
	Use:                   "stop <name>",
	Short:                 "Stop a running database",
	Example:               "  orb db stop mydb",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		dbMgr, err = database.NewService()
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return dbMgr.Stop(args[0])
	},
}

var dbDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a database and its data",
	Long: `Delete a database container and optionally keep its data.

By default, both the container and data are removed.
Use --keep-data to preserve the data directory.`,
	Example: `  orb db delete mydb
  orb db delete mydb --keep-data`,
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		dbMgr, err = database.NewService()
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		keepData, _ := cmd.Flags().GetBool("keep-data")
		return dbMgr.Delete(args[0], keepData)
	},
}

var dbLogsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "View database logs",
	Example: `  orb db logs mydb
  orb db logs mydb -f
  orb db logs mydb -n 50`,
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		dbMgr, err = database.NewService()
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		follow, _ := cmd.Flags().GetBool("follow")
		lines, _ := cmd.Flags().GetInt("lines")
		return dbMgr.Logs(args[0], follow, lines)
	},
}

var dbInfoCmd = &cobra.Command{
	Use:                   "info <name>",
	Short:                 "Show database connection info",
	Example:               "  orb db info mydb",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		dbMgr, err = database.NewService()
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := dbMgr.GetConfig(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Database: %s\n", cfg.Name)
		fmt.Printf("Type:     %s\n", cfg.Type)
		fmt.Printf("Port:     %s\n", cfg.Port)
		fmt.Printf("Data:     %s\n", cfg.DataDir)
		fmt.Println()

		switch cfg.Type {
		case "postgres":
			fmt.Printf("Connect:  psql -h localhost -p %s -U postgres\n", cfg.Port)
			fmt.Println("Password: orb")
		case "mysql":
			fmt.Printf("Connect:  mysql -h 127.0.0.1 -P %s -u root -p\n", cfg.Port)
			fmt.Println("Password: orb")
		case "redis":
			fmt.Printf("Connect:  redis-cli -p %s\n", cfg.Port)
		case "mongodb":
			fmt.Printf("Connect:  mongosh --port %s -u root -p orb\n", cfg.Port)
		}

		return nil
	},
}

func init() {
	// Expose command flags
	dbExposeCmd.Flags().StringP("port", "p", "", "Port to expose (defaults to standard port for db type)")
	dbExposeCmd.Flags().StringP("access", "a", "private", "Access level: public, private, or group name")
	dbExposeCmd.Flags().StringP("expires", "e", "", "Auto-revoke group access after duration (e.g., 1h, 24h)")

	// Create command flags
	dbCreateCmd.Flags().StringP("port", "p", "", "Port to bind (defaults to standard port for db type)")

	// Delete command flags
	dbDeleteCmd.Flags().Bool("keep-data", false, "Keep the data directory when deleting")

	// Logs command flags
	dbLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	dbLogsCmd.Flags().IntP("lines", "n", 100, "Number of lines to show")

	// Add all subcommands
	dbCmd.AddCommand(dbExposeCmd)
	dbCmd.AddCommand(dbTypesCmd)
	dbCmd.AddCommand(dbCreateCmd)
	dbCmd.AddCommand(dbListManagedCmd)
	dbCmd.AddCommand(dbStartCmd)
	dbCmd.AddCommand(dbStopCmd)
	dbCmd.AddCommand(dbDeleteCmd)
	dbCmd.AddCommand(dbLogsCmd)
	dbCmd.AddCommand(dbInfoCmd)
}
