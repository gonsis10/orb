# orb

A CLI tool for managing Cloudflare Tunnel ingress rules. Easily expose and manage local services through Cloudflare Tunnel with simple commands.

## Features

- **Expose local services** through Cloudflare Tunnel with custom subdomains
- **Manage DNS routes** automatically via Cloudflare API
- **Update port mappings** for existing subdomains
- **List all exposed services** at a glance
- **Port validation** ensures services are running before exposure

## Prerequisites

- Go 1.25.5 or later
- [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) (`cloudflared`) installed and configured
- Cloudflare API token with DNS edit permissions
- A configured `cloudflared` config file

## Installation

All `orb` configuration lives in `~/.config/orb/` for easy management.

### Method 1: Using `go install` (Recommended)

```bash
# 1. Clone and install the binary
git clone https://github.com/yourusername/orb.git
cd orb
go install

# 2. Add Go bin to your PATH (if not already configured)
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc

# 3. Create config directory and environment file
mkdir -p ~/.config/orb
nano ~/.config/orb/.env
```

Add your configuration to `~/.config/orb/.env`:
```bash
DOMAIN=yourdomain.com
CONFIG_PATH=/path/to/cloudflared/config.yml
CLOUDFLARE_API_TOKEN=your_api_token
CLOUDFLARE_ZONE_ID=your_zone_id
```

**Verify installation:**
```bash
orb --help
```

### Method 2: Build and Install Manually

```bash
# 1. Build and install binary
go build -o orb
sudo mv orb /usr/local/bin/

# 2. Create config directory and environment file
mkdir -p ~/.config/orb
nano ~/.config/orb/.env
```

Add your configuration to `~/.config/orb/.env` (same as above).

## Uninstallation

```bash
# Remove the binary
rm $(which orb)

# Remove all configuration
rm -rf ~/.config/orb
```

## Configuration

`orb` loads configuration from `~/.config/orb/.env` (created during installation).

**Configuration Priority:**
1. `~/.config/orb/.env`
2. Environment variables already set in your shell

### Getting Your Cloudflare Credentials

1. **API Token**: Create one at [Cloudflare Dashboard](https://dash.cloudflare.com/profile/api-tokens) with DNS edit permissions
2. **Zone ID**: Found in your domain's Overview tab in the Cloudflare Dashboard

## Usage

### Expose a Local Service

Expose a local port through a subdomain:

```bash
orb tunnel expose api 8080
```

This creates:
- DNS CNAME record: `api.yourdomain.com` → Cloudflare Tunnel
- Ingress rule in cloudflared config
- Restarts cloudflared service

The service at `localhost:8080` is now accessible at `https://api.yourdomain.com`

### Remove an Exposed Service

```bash
orb tunnel unexpose api
```

Removes the DNS record and ingress rule for the subdomain.

### Update Port Mapping

```bash
orb tunnel update api 9090
```

Changes `api.yourdomain.com` to point to `localhost:9090` instead.

### List All Exposed Services

```bash
orb tunnel list
```

Output:
```
Exposed services:
  https://api.yourdomain.com            → http://localhost:8080
  https://dashboard.yourdomain.com      → http://localhost:3000
```

## How It Works

1. **Validation**: Checks subdomain format and verifies the port is listening
2. **Config Update**: Modifies your `cloudflared` YAML configuration
3. **DNS Management**: Creates/updates DNS records via Cloudflare API
4. **Service Restart**: Restarts `cloudflared` to apply changes

## Project Structure

**Source code:**
```
orb/
├── cmd/                      # CLI commands (Cobra)
│   ├── root.go              # Root command
│   └── tunnel.go            # Tunnel subcommands
├── internal/
│   ├── dns/                 # Cloudflare DNS client
│   │   └── client.go
│   └── tunnel/              # Tunnel management logic
│       ├── config.go        # Config file management
│       ├── service.go       # Business logic
│       └── validation.go    # Input validation
├── main.go                  # Entry point
└── go.mod
```

**Runtime configuration:**
```
~/.config/orb/
└── .env                     # Environment variables (API tokens, domain, etc.)
```

## Development

### Build

```bash
go build -o orb
```

### Run Without Installing

```bash
go run . tunnel expose api 8080
```

## Troubleshooting

### "Nothing listening on 127.0.0.1:PORT"

Start your service first before exposing it:
```bash
# Start your service
./your-service &

# Then expose it
orb tunnel expose api 8080
```

### "Permission denied" errors

The cloudflared config file might require sudo access:
```bash
sudo orb tunnel expose api 8080
```

### "DOMAIN environment variable is required"

Make sure your `.env` file exists at `~/.config/orb/.env` and contains all required variables:
```bash
# Check if config exists
cat ~/.config/orb/.env

# Or create it if missing
mkdir -p ~/.config/orb
nano ~/.config/orb/.env
```

Alternatively, set them in your shell:
```bash
export DOMAIN=yourdomain.com
export CONFIG_PATH=/etc/cloudflared/config.yml
export CLOUDFLARE_API_TOKEN=your_token
export CLOUDFLARE_ZONE_ID=your_zone_id
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI framework
- Uses [cloudflare-go](https://github.com/cloudflare/cloudflare-go) for API interaction
- Powered by [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)
