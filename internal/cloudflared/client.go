package cloudflared

import (
	"fmt"
	"os/exec"
)

type Client struct{}

func New() *Client {
	return &Client{}
}

func (c *Client) CreateDNSRoute(tunnelID, hostname string) error {
	cmd := exec.Command("cloudflared", "tunnel", "route", "dns", "--overwrite-dns", tunnelID, hostname)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create DNS route: %w\nOutput: %s", err, string(output))
	}
	return nil
}
