package main

const (
	Domain = "simoonsong.com"
	CloudflaredSvc = "cloudflared.service"
	ConfigPath = "/etc/cloudflared/config.yml"
	LockPath = "/tmp/orb-cloudflared.lock"
)

type IngressRule struct {
	Hostname string `yaml:"hostname,omitempty"`
	Service  string `yaml:"service"`
}

type TunnelConfig struct {
	Tunnel string `yaml:"tunnel"`
	CredentialsFile string `yaml:"credentials-file"`
	Ingress []IngressRule `yaml:"ingress"`
}