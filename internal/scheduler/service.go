package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
)

// Schedule represents a scheduled task
type Schedule struct {
	Name      string    `json:"name"`
	Cron      string    `json:"cron"`
	Command   string    `json:"command"`
	CreatedAt time.Time `json:"created_at"`
}

// Service manages scheduled tasks
type Service struct {
	configPath string
	schedules  map[string]Schedule
}

// NewService creates a new scheduler service
func NewService() (*Service, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config dir: %w", err)
	}

	orbDir := filepath.Join(configDir, "orb")
	if err := os.MkdirAll(orbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	s := &Service{
		configPath: filepath.Join(orbDir, "schedules.json"),
		schedules:  make(map[string]Schedule),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

// load reads schedules from the config file
func (s *Service) load() error {
	data, err := os.ReadFile(s.configPath)
	if os.IsNotExist(err) {
		return nil // No schedules yet
	}
	if err != nil {
		return fmt.Errorf("failed to read schedules: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, &s.schedules); err != nil {
		return fmt.Errorf("failed to parse schedules: %w", err)
	}

	return nil
}

// save writes schedules to the config file
func (s *Service) save() error {
	data, err := json.MarshalIndent(s.schedules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedules: %w", err)
	}

	if err := os.WriteFile(s.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write schedules: %w", err)
	}

	return nil
}

// Add creates a new scheduled task
func (s *Service) Add(name, cron, command string) error {
	// Validate name
	if name == "" {
		return fmt.Errorf("schedule name cannot be empty")
	}
	if strings.ContainsAny(name, " \t\n") {
		return fmt.Errorf("schedule name cannot contain whitespace")
	}

	// Check for duplicates
	if _, exists := s.schedules[name]; exists {
		return fmt.Errorf("schedule %q already exists, use 'orb schedule remove %s' first", name, name)
	}

	// Validate cron expression (basic validation)
	if err := validateCron(cron); err != nil {
		return err
	}

	// Validate command exists (if it's a file path)
	if strings.HasPrefix(command, "/") || strings.HasPrefix(command, "./") || strings.HasPrefix(command, "~/") {
		cmdPath := command
		if strings.HasPrefix(cmdPath, "~/") {
			home, _ := os.UserHomeDir()
			cmdPath = filepath.Join(home, cmdPath[2:])
		}
		// Extract just the executable (first word)
		parts := strings.Fields(cmdPath)
		if len(parts) > 0 {
			if _, err := os.Stat(parts[0]); os.IsNotExist(err) {
				return fmt.Errorf("command not found: %s", parts[0])
			}
		}
	}

	// Create schedule entry
	schedule := Schedule{
		Name:      name,
		Cron:      cron,
		Command:   command,
		CreatedAt: time.Now(),
	}

	// Add to crontab
	if err := s.addToCrontab(schedule); err != nil {
		return fmt.Errorf("failed to add to crontab: %w", err)
	}

	// Save to config
	s.schedules[name] = schedule
	if err := s.save(); err != nil {
		// Rollback crontab
		s.removeFromCrontab(name)
		return err
	}

	fmt.Printf("✓ Schedule %q added\n", name)
	fmt.Printf("  Cron: %s\n", cron)
	fmt.Printf("  Command: %s\n", command)
	fmt.Printf("  Next run: %s\n", describeNextRun(cron))

	return nil
}

// Remove deletes a scheduled task
func (s *Service) Remove(name string) error {
	if _, exists := s.schedules[name]; !exists {
		return fmt.Errorf("schedule %q not found", name)
	}

	// Remove from crontab
	if err := s.removeFromCrontab(name); err != nil {
		return fmt.Errorf("failed to remove from crontab: %w", err)
	}

	// Remove from config
	delete(s.schedules, name)
	if err := s.save(); err != nil {
		return err
	}

	fmt.Printf("✓ Schedule %q removed\n", name)
	return nil
}

// List shows all scheduled tasks
func (s *Service) List() error {
	if len(s.schedules) == 0 {
		fmt.Println("No scheduled tasks")
		fmt.Println("\nUse 'orb schedule add <name> <cron> <command>' to create one")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("Name", "Cron", "Command", "Created")

	for _, sched := range s.schedules {
		if err := table.Append(
			sched.Name,
			sched.Cron,
			truncate(sched.Command, 40),
			sched.CreatedAt.Format("2006-01-02"),
		); err != nil {
			return fmt.Errorf("failed to add table row: %w", err)
		}
	}

	fmt.Println("\nScheduled tasks:")
	if err := table.Render(); err != nil {
		return fmt.Errorf("failed to render table: %w", err)
	}

	return nil
}

// addToCrontab adds a schedule to the user's crontab
func (s *Service) addToCrontab(sched Schedule) error {
	// Get current crontab
	current, _ := exec.Command("crontab", "-l").Output()

	// Build new entry with marker comment
	marker := fmt.Sprintf("# orb-schedule: %s", sched.Name)
	entry := fmt.Sprintf("%s\n%s %s\n", marker, sched.Cron, sched.Command)

	// Append to crontab
	newCrontab := string(current) + entry

	// Write new crontab
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, output)
	}

	return nil
}

// removeFromCrontab removes a schedule from the user's crontab
func (s *Service) removeFromCrontab(name string) error {
	// Get current crontab
	current, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		return nil // No crontab exists
	}

	// Remove lines with our marker
	marker := fmt.Sprintf("# orb-schedule: %s", name)
	lines := strings.Split(string(current), "\n")
	var newLines []string
	skipNext := false

	for _, line := range lines {
		if strings.Contains(line, marker) {
			skipNext = true
			continue
		}
		if skipNext {
			skipNext = false
			continue
		}
		newLines = append(newLines, line)
	}

	// Write new crontab
	newCrontab := strings.Join(newLines, "\n")
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, output)
	}

	return nil
}

// validateCron performs basic cron expression validation
func validateCron(cron string) error {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return fmt.Errorf("invalid cron expression: expected 5 fields (minute hour day month weekday), got %d", len(fields))
	}

	// Basic check - each field should have valid characters
	for _, field := range fields {
		for _, c := range field {
			if !strings.ContainsRune("0123456789*,-/", c) {
				return fmt.Errorf("invalid character %q in cron expression", c)
			}
		}
	}

	return nil
}

// describeNextRun gives a human-readable description of when the cron will run
func describeNextRun(cron string) string {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return "invalid cron"
	}

	min, hour, day, month, weekday := fields[0], fields[1], fields[2], fields[3], fields[4]

	// Handle common patterns
	if min == "0" && hour == "*" && day == "*" && month == "*" && weekday == "*" {
		return "every hour"
	}
	if min != "*" && hour != "*" && day == "*" && month == "*" && weekday == "*" {
		return fmt.Sprintf("daily at %s:%s", hour, padZero(min))
	}
	if weekday != "*" && day == "*" {
		return fmt.Sprintf("weekly on %s at %s:%s", weekdayName(weekday), hour, padZero(min))
	}

	return "see cron expression"
}

func padZero(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func weekdayName(day string) string {
	days := map[string]string{
		"0": "Sunday", "1": "Monday", "2": "Tuesday", "3": "Wednesday",
		"4": "Thursday", "5": "Friday", "6": "Saturday", "7": "Sunday",
	}
	if name, ok := days[day]; ok {
		return name
	}
	return day
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
