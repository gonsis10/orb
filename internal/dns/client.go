// Package dns provides a client for managing Cloudflare DNS records
// and cloudflared service operations.
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
	api    *cloudflare.API
	zoneID string
}

// New creates a new Cloudflare DNS client
func New() (*Client, error) {
	api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
	if err != nil {
		return nil, fmt.Errorf("failed to create cloudflare client: %w", err)
	}

	return &Client{
		api:    api,
		zoneID: os.Getenv("CLOUDFLARE_ZONE_ID"),
	}, nil
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
func (c *Client) RestartCloudflaredService(tunnelID, hostname string) error {
	cmd := exec.Command("sudo", "systemctl", "restart", "cloudflared")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart cloudflared service: %w\nOutput: %s", err, string(output))
	}
	return nil
}
