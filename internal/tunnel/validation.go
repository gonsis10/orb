package tunnel

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"time"
)

var (
	subdomainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	portRe      = regexp.MustCompile(`^\d{1,5}$`)
)

func ValidateSubdomain(s string) error {
	if !subdomainRe.MatchString(s) {
		return errors.New("invalid subdomain: use lowercase letters, digits, and hyphens (must start/end with alphanumeric)")
	}
	return nil
}

func ValidatePort(p string) error {
	if !portRe.MatchString(p) {
		return errors.New("invalid port: must be a number between 1-65535")
	}
	return nil
}

func EnsurePortListening(port string) error {
	addr := "127.0.0.1:" + port
	conn, err := net.DialTimeout("tcp", addr, 800*time.Millisecond)
	if err != nil {
		return fmt.Errorf("nothing listening on %s - start your service first", addr)
	}
	conn.Close()
	return nil
}
