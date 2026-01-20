package cmd

import (
	"fmt"

	"orb/internal/tunnel"

	"github.com/spf13/cobra"
)

var dbSvc *tunnel.Service

// Database types with their default ports
var dbDefaults = map[string]struct {
	port        string
	serviceType string
	description string
}{
	"postgres":  {port: "5432", serviceType: "tcp", description: "PostgreSQL"},
	"mysql":     {port: "3306", serviceType: "tcp", description: "MySQL/MariaDB"},
	"redis":     {port: "6379", serviceType: "tcp", description: "Redis"},
	"mongodb":   {port: "27017", serviceType: "tcp", description: "MongoDB"},
	"memcached": {port: "11211", serviceType: "tcp", description: "Memcached"},
	"mssql":     {port: "1433", serviceType: "tcp", description: "Microsoft SQL Server"},
	"clickhouse": {port: "9000", serviceType: "tcp", description: "ClickHouse"},
	"cassandra": {port: "9042", serviceType: "tcp", description: "Cassandra"},
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Expose databases through Cloudflare Tunnel",
	Long: `Expose local databases securely through Cloudflare Tunnel.

Supported databases: postgres, mysql, redis, mongodb, memcached, mssql, clickhouse, cassandra

Databases are exposed as TCP services with private access by default.`,
	Example: `  orb db expose postgres mydb
  orb db expose mysql mydb --port 3307
  orb db expose redis cache --access team`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		dbSvc, err = tunnel.NewService()
		return err
	},
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

var dbListCmd = &cobra.Command{
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

func init() {
	dbExposeCmd.Flags().StringP("port", "p", "", "Port to expose (defaults to standard port for db type)")
	dbExposeCmd.Flags().StringP("access", "a", "private", "Access level: public, private, or group name")
	dbExposeCmd.Flags().StringP("expires", "e", "", "Auto-revoke group access after duration (e.g., 1h, 24h)")

	dbCmd.AddCommand(dbExposeCmd)
	dbCmd.AddCommand(dbListCmd)
}
