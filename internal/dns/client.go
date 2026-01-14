package dns

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cloudflare/cloudflare-go"
)

// Client wraps the Cloudflare API for DNS management
type Client struct {
	api       *cloudflare.API
	zoneID    string
	accountID string
}

// New creates a new Cloudflare DNS client
func New() (*Client, error) {
	api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
	if err != nil {
		return nil, fmt.Errorf("failed to create cloudflare client: %w", err)
	}

	return &Client{
		api:       api,
		zoneID:    os.Getenv("CLOUDFLARE_ZONE_ID"),
		accountID: os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
	}, nil
}

// GetTunnelName retrieves the tunnel name from the Cloudflare API using the tunnel ID
func (c *Client) GetTunnelName(tunnelID string) (string, error) {
	ctx := context.Background()

	tunnel, err := c.api.GetTunnel(ctx, cloudflare.AccountIdentifier(c.accountID), tunnelID)
	if err != nil {
		return "", fmt.Errorf("failed to get tunnel: %w", err)
	}

	return tunnel.Name, nil
}

// CreateDNSRoute creates a CNAME DNS record for the tunnel
func (c *Client) CreateDNSRoute(tunnelID, hostname string) error {
	ctx := context.Background()

	target := fmt.Sprintf("%s.cfargotunnel.com", tunnelID)

	params := cloudflare.CreateDNSRecordParams{
		Type:    "CNAME",
		Name:    hostname,
		Content: target,
		Proxied: cloudflare.BoolPtr(true),
		TTL:     1,
	}

	_, err := c.api.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(c.zoneID), params)
	if err != nil {
		return fmt.Errorf("failed to create DNS record: %w", err)
	}

	return nil
}

// RemoveDNSRoute removes CNAME DNS record for the tunnel
func (c *Client) RemoveDNSRoute(tunnelID, hostname string) error {
	ctx := context.Background()

	records, _, err := c.api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(c.zoneID), cloudflare.ListDNSRecordsParams{
		Name: hostname,
		Type: "CNAME",
	})
	if err != nil {
		return fmt.Errorf("failed to list DNS records: %w", err)
	}

	if len(records) == 0 {
		return fmt.Errorf("no DNS record found for hostname: %s", hostname)
	}

	for _, record := range records {
		err := c.api.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(c.zoneID), record.ID)
		if err != nil {
			return fmt.Errorf("failed to delete DNS record: %w", err)
		}
	}

	return nil
}

// FlushLocalDNSCache flushes the local DNS cache to pick up new DNS records
func (c *Client) FlushLocalDNSCache() error {
	// Try systemd-resolved first (Ubuntu/Debian)
	cmd := exec.Command("sudo", "systemd-resolve", "--flush-caches")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Try resolvectl (newer systemd)
	cmd = exec.Command("sudo", "resolvectl", "flush-caches")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// If both fail, it's not critical - DNS will eventually refresh
	return nil
}

// RestartCloudflaredService restarts the cloudflared service to apply DNS changes
func (c *Client) RestartCloudflaredService(tunnelName, hostname string) error {
	serviceName := fmt.Sprintf("cloudflared-%s", tunnelName)
	cmd := exec.Command("sudo", "systemctl", "restart", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart %s service: %w\nOutput: %s", serviceName, err, string(output))
	}
	return nil
}

// GetServiceStatus returns the status of the cloudflared service
func (c *Client) GetServiceStatus(tunnelName string) (string, error) {
	serviceName := fmt.Sprintf("cloudflared-%s", tunnelName)
	cmd := exec.Command("systemctl", "status", serviceName, "--no-pager")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// systemctl status returns exit code 3 if service is not running, but still outputs status
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
			return string(output), nil
		}
		return "", fmt.Errorf("failed to get %s service status: %w\nOutput: %s", serviceName, err, string(output))
	}
	return string(output), nil
}

// GetServiceLogs returns the logs of the cloudflared service, optionally filtered by hostname
func (c *Client) GetServiceLogs(tunnelName string, lines int, hostname string) (string, error) {
	serviceName := fmt.Sprintf("cloudflared-%s", tunnelName)
	args := []string{"-u", serviceName, "--no-pager", "-n", fmt.Sprintf("%d", lines)}

	if hostname != "" {
		args = append(args, "--grep", hostname)
	}

	cmd := exec.Command("journalctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get %s service logs: %w\nOutput: %s", serviceName, err, string(output))
	}
	return string(output), nil
}

// FollowServiceLogs follows the logs of the cloudflared service in real-time
func (c *Client) FollowServiceLogs(tunnelName string, hostname string) error {
	serviceName := fmt.Sprintf("cloudflared-%s", tunnelName)
	args := []string{"-u", serviceName, "-f"}

	if hostname != "" {
		args = append(args, "--grep", hostname)
	}

	cmd := exec.Command("journalctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// CreateAccessPolicy creates a Cloudflare Access policy for a hostname
// accessLevel can be "public", "private", or a group name
func (c *Client) CreateAccessPolicy(hostname, accessLevel, userEmail string) error {
	ctx := context.Background()

	// If access level is public, don't create a policy
	if accessLevel == "public" {
		return nil
	}

	// Build include rules based on access level
	var include []any
	if accessLevel == "private" {
		// Private: only the user's email
		include = []any{
			cloudflare.AccessGroupEmail{Email: struct {
				Email string `json:"email"`
			}{Email: userEmail}},
		}
	} else {
		// Assume it's a group name - look up the group by name
		groups, _, err := c.api.ListAccessGroups(ctx, cloudflare.AccountIdentifier(c.accountID), cloudflare.ListAccessGroupsParams{})
		if err != nil {
			return fmt.Errorf("failed to list access groups: %w", err)
		}

		var groupID string
		for _, group := range groups {
			if group.Name == accessLevel {
				groupID = group.ID
				break
			}
		}

		if groupID == "" {
			return fmt.Errorf("access group %q not found - create it in Cloudflare Zero Trust dashboard first", accessLevel)
		}

		// Reference the existing group
		include = []any{
			cloudflare.AccessGroupAccessGroup{Group: struct {
				ID string `json:"id"`
			}{ID: groupID}},
		}
	}

	// Create the access application
	createdApp, err := c.api.CreateAccessApplication(ctx, cloudflare.AccountIdentifier(c.accountID), cloudflare.CreateAccessApplicationParams{
		Name:   fmt.Sprintf("orb-%s", hostname),
		Domain: hostname,
		Type:   "self_hosted",
	})
	if err != nil {
		return fmt.Errorf("failed to create access application: %w", err)
	}

	// Create the access policy
	_, err = c.api.CreateAccessPolicy(ctx, cloudflare.AccountIdentifier(c.accountID), cloudflare.CreateAccessPolicyParams{
		ApplicationID: createdApp.ID,
		Name:          fmt.Sprintf("orb-%s-policy", hostname),
		Decision:      "allow",
		Include:       include,
		Precedence:    1,
	})
	if err != nil {
		return fmt.Errorf("failed to create access policy: %w", err)
	}

	return nil
}

// RemoveAccessPolicy removes the Cloudflare Access policy for a hostname
func (c *Client) RemoveAccessPolicy(hostname string) error {
	ctx := context.Background()

	// List all access applications
	apps, _, err := c.api.ListAccessApplications(ctx, cloudflare.AccountIdentifier(c.accountID), cloudflare.ListAccessApplicationsParams{})
	if err != nil {
		return fmt.Errorf("failed to list access applications: %w", err)
	}

	// Find the application for this hostname
	appName := fmt.Sprintf("orb-%s", hostname)
	for _, app := range apps {
		if app.Name == appName {
			// Delete the application (this also deletes associated policies)
			err := c.api.DeleteAccessApplication(ctx, cloudflare.AccountIdentifier(c.accountID), app.ID)
			if err != nil {
				return fmt.Errorf("failed to delete access application: %w", err)
			}
			return nil
		}
	}

	// Not found is not an error
	return nil
}

// CreateAccessGroup creates a new Access group with email addresses
func (c *Client) CreateAccessGroup(groupName, emails string) error {
	ctx := context.Background()

	// Parse comma-separated emails
	emailList := []string{}
	for _, email := range strings.Split(emails, ",") {
		emailList = append(emailList, strings.TrimSpace(email))
	}

	// Build include rules with email addresses
	var include []any
	for _, email := range emailList {
		include = append(include, cloudflare.AccessGroupEmail{Email: struct {
			Email string `json:"email"`
		}{Email: email}})
	}

	// Create the access group
	_, err := c.api.CreateAccessGroup(ctx, cloudflare.AccountIdentifier(c.accountID), cloudflare.CreateAccessGroupParams{
		Name:    groupName,
		Include: include,
	})
	if err != nil {
		return fmt.Errorf("failed to create access group: %w", err)
	}

	fmt.Printf("✔ Created Access group %q with %d email(s)\n", groupName, len(emailList))
	return nil
}

// ListAccessGroupsFormatted lists all Access groups in a formatted table
func (c *Client) ListAccessGroupsFormatted() error {
	ctx := context.Background()

	groups, _, err := c.api.ListAccessGroups(ctx, cloudflare.AccountIdentifier(c.accountID), cloudflare.ListAccessGroupsParams{})
	if err != nil {
		return fmt.Errorf("failed to list access groups: %w", err)
	}

	if len(groups) == 0 {
		fmt.Println("No Access groups found")
		return nil
	}

	fmt.Printf("\nAccess Groups (%d):\n", len(groups))
	for _, group := range groups {
		fmt.Printf("  • %s (ID: %s)\n", group.Name, group.ID)
	}

	return nil
}

// DeleteAccessGroup deletes an Access group by name
func (c *Client) DeleteAccessGroup(groupName string) error {
	ctx := context.Background()

	// Find the group by name
	groups, _, err := c.api.ListAccessGroups(ctx, cloudflare.AccountIdentifier(c.accountID), cloudflare.ListAccessGroupsParams{})
	if err != nil {
		return fmt.Errorf("failed to list access groups: %w", err)
	}

	var groupID string
	for _, group := range groups {
		if group.Name == groupName {
			groupID = group.ID
			break
		}
	}

	if groupID == "" {
		return fmt.Errorf("access group %q not found", groupName)
	}

	// Delete the group
	err = c.api.DeleteAccessGroup(ctx, cloudflare.AccountIdentifier(c.accountID), groupID)
	if err != nil {
		return fmt.Errorf("failed to delete access group: %w", err)
	}

	fmt.Printf("✔ Deleted Access group %q\n", groupName)
	return nil
}
