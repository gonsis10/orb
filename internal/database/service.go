package database

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DBConfig represents a managed database instance
type DBConfig struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Port        string `json:"port"`
	ContainerID string `json:"container_id"`
	DataDir     string `json:"data_dir"`
}

// DBType contains configuration for a database type
type DBType struct {
	Image       string
	DefaultPort string
	EnvVars     map[string]string
	DataPath    string // Path inside container for data
}

// SupportedDBs maps database types to their configurations
var SupportedDBs = map[string]DBType{
	"postgres": {
		Image:       "postgres:16-alpine",
		DefaultPort: "5432",
		EnvVars:     map[string]string{"POSTGRES_PASSWORD": "orb"},
		DataPath:    "/var/lib/postgresql/data",
	},
	"mysql": {
		Image:       "mysql:8",
		DefaultPort: "3306",
		EnvVars:     map[string]string{"MYSQL_ROOT_PASSWORD": "orb"},
		DataPath:    "/var/lib/mysql",
	},
	"redis": {
		Image:       "redis:7-alpine",
		DefaultPort: "6379",
		EnvVars:     map[string]string{},
		DataPath:    "/data",
	},
	"mongodb": {
		Image:       "mongo:7",
		DefaultPort: "27017",
		EnvVars:     map[string]string{"MONGO_INITDB_ROOT_USERNAME": "root", "MONGO_INITDB_ROOT_PASSWORD": "orb"},
		DataPath:    "/data/db",
	},
}

// Service manages database containers
type Service struct {
	configDir string
	dataDir   string
}

// NewService creates a new database service
func NewService() (*Service, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "orb", "databases")
	dataDir := filepath.Join(homeDir, ".local", "share", "orb", "databases")

	// Ensure directories exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &Service{
		configDir: configDir,
		dataDir:   dataDir,
	}, nil
}

// checkDocker verifies Docker is available
func (s *Service) checkDocker() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not running or not installed")
	}
	return nil
}

// Create creates a new database container
func (s *Service) Create(dbType, name, port string) error {
	if err := s.checkDocker(); err != nil {
		return err
	}

	dbConfig, ok := SupportedDBs[dbType]
	if !ok {
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	// Check if database already exists
	if _, err := s.GetConfig(name); err == nil {
		return fmt.Errorf("database %q already exists", name)
	}

	if port == "" {
		port = dbConfig.DefaultPort
	}

	// Create data directory for this database
	dataPath := filepath.Join(s.dataDir, name)
	if err := os.MkdirAll(dataPath, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Build docker run command
	containerName := fmt.Sprintf("orb-db-%s", name)
	args := []string{
		"run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("127.0.0.1:%s:%s", port, dbConfig.DefaultPort),
		"-v", fmt.Sprintf("%s:%s", dataPath, dbConfig.DataPath),
		"--restart", "unless-stopped",
	}

	// Add environment variables
	for key, value := range dbConfig.EnvVars {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	args = append(args, dbConfig.Image)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create container: %w\n%s", err, string(output))
	}

	containerID := strings.TrimSpace(string(output))

	// Save config
	config := DBConfig{
		Name:        name,
		Type:        dbType,
		Port:        port,
		ContainerID: containerID,
		DataDir:     dataPath,
	}

	if err := s.saveConfig(config); err != nil {
		// Cleanup container on failure
		exec.Command("docker", "rm", "-f", containerName).Run()
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✔ Created %s database %q\n", dbType, name)
	fmt.Printf("  Port: %s\n", port)
	fmt.Printf("  Data: %s\n", dataPath)

	// Print connection info
	s.printConnectionInfo(dbType, name, port)

	return nil
}

// printConnectionInfo prints how to connect to the database
func (s *Service) printConnectionInfo(dbType, name, port string) {
	fmt.Println()
	switch dbType {
	case "postgres":
		fmt.Printf("  Connect: psql -h localhost -p %s -U postgres\n", port)
		fmt.Println("  Password: orb")
	case "mysql":
		fmt.Printf("  Connect: mysql -h 127.0.0.1 -P %s -u root -p\n", port)
		fmt.Println("  Password: orb")
	case "redis":
		fmt.Printf("  Connect: redis-cli -p %s\n", port)
	case "mongodb":
		fmt.Printf("  Connect: mongosh --port %s -u root -p orb\n", port)
	}
}

// List lists all managed databases
func (s *Service) List() error {
	configs, err := s.getAllConfigs()
	if err != nil {
		return err
	}

	if len(configs) == 0 {
		fmt.Println("No databases found")
		fmt.Println("\nCreate one with: orb db create <type> <name>")
		return nil
	}

	fmt.Printf("\nManaged databases (%d):\n\n", len(configs))
	fmt.Printf("  %-15s %-12s %-8s %-12s\n", "NAME", "TYPE", "PORT", "STATUS")
	fmt.Printf("  %-15s %-12s %-8s %-12s\n", "----", "----", "----", "------")

	for _, cfg := range configs {
		status := s.getContainerStatus(cfg.Name)
		fmt.Printf("  %-15s %-12s %-8s %-12s\n", cfg.Name, cfg.Type, cfg.Port, status)
	}

	return nil
}

// getContainerStatus checks if the container is running
func (s *Service) getContainerStatus(name string) string {
	containerName := fmt.Sprintf("orb-db-%s", name)
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Status}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// Start starts a stopped database
func (s *Service) Start(name string) error {
	if err := s.checkDocker(); err != nil {
		return err
	}

	cfg, err := s.GetConfig(name)
	if err != nil {
		return err
	}

	containerName := fmt.Sprintf("orb-db-%s", name)
	cmd := exec.Command("docker", "start", containerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start database: %w", err)
	}

	fmt.Printf("✔ Started database %q on port %s\n", name, cfg.Port)
	return nil
}

// Stop stops a running database
func (s *Service) Stop(name string) error {
	if err := s.checkDocker(); err != nil {
		return err
	}

	if _, err := s.GetConfig(name); err != nil {
		return err
	}

	containerName := fmt.Sprintf("orb-db-%s", name)
	cmd := exec.Command("docker", "stop", containerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop database: %w", err)
	}

	fmt.Printf("✔ Stopped database %q\n", name)
	return nil
}

// Delete removes a database and its data
func (s *Service) Delete(name string, keepData bool) error {
	if err := s.checkDocker(); err != nil {
		return err
	}

	cfg, err := s.GetConfig(name)
	if err != nil {
		return err
	}

	containerName := fmt.Sprintf("orb-db-%s", name)

	// Remove container
	cmd := exec.Command("docker", "rm", "-f", containerName)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to remove container: %v\n", err)
	}

	// Remove data unless --keep-data
	if !keepData {
		if err := os.RemoveAll(cfg.DataDir); err != nil {
			fmt.Printf("Warning: failed to remove data directory: %v\n", err)
		}
	}

	// Remove config
	configPath := filepath.Join(s.configDir, name+".json")
	os.Remove(configPath)

	fmt.Printf("✔ Deleted database %q\n", name)
	if keepData {
		fmt.Printf("  Data preserved at: %s\n", cfg.DataDir)
	}
	return nil
}

// Logs shows database logs
func (s *Service) Logs(name string, follow bool, lines int) error {
	if err := s.checkDocker(); err != nil {
		return err
	}

	if _, err := s.GetConfig(name); err != nil {
		return err
	}

	containerName := fmt.Sprintf("orb-db-%s", name)
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, "--tail", fmt.Sprintf("%d", lines), containerName)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetConfig retrieves a database configuration
func (s *Service) GetConfig(name string) (*DBConfig, error) {
	configPath := filepath.Join(s.configDir, name+".json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("database %q not found", name)
		}
		return nil, err
	}

	var cfg DBConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// saveConfig saves a database configuration
func (s *Service) saveConfig(cfg DBConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(s.configDir, cfg.Name+".json")
	return os.WriteFile(configPath, data, 0600)
}

// Shell opens an interactive shell to the database
func (s *Service) Shell(name string) error {
	if err := s.checkDocker(); err != nil {
		return err
	}

	cfg, err := s.GetConfig(name)
	if err != nil {
		return err
	}

	// Check if container is running
	status := s.getContainerStatus(name)
	if status != "running" {
		return fmt.Errorf("database %q is not running (status: %s)", name, status)
	}

	containerName := fmt.Sprintf("orb-db-%s", name)

	var cmd *exec.Cmd
	switch cfg.Type {
	case "postgres":
		// Use psql inside the container
		cmd = exec.Command("docker", "exec", "-it", containerName,
			"psql", "-U", "postgres")
	case "mysql":
		// Use mysql inside the container
		cmd = exec.Command("docker", "exec", "-it", containerName,
			"mysql", "-u", "root", "-porb")
	case "redis":
		// Use redis-cli inside the container
		cmd = exec.Command("docker", "exec", "-it", containerName,
			"redis-cli")
	case "mongodb":
		// Use mongosh inside the container
		cmd = exec.Command("docker", "exec", "-it", containerName,
			"mongosh", "-u", "root", "-p", "orb", "--authenticationDatabase", "admin")
	default:
		return fmt.Errorf("shell not supported for database type: %s", cfg.Type)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// getAllConfigs retrieves all database configurations
func (s *Service) getAllConfigs() ([]DBConfig, error) {
	entries, err := os.ReadDir(s.configDir)
	if err != nil {
		return nil, err
	}

	var configs []DBConfig
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			name := strings.TrimSuffix(entry.Name(), ".json")
			cfg, err := s.GetConfig(name)
			if err != nil {
				continue
			}
			configs = append(configs, *cfg)
		}
	}
	return configs, nil
}
