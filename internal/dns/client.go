package dns

import (
	"context"
	"fmt"
	"os"
	"os/exec"

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
