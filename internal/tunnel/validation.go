package tunnel

import (
	"fmt"
	"regexp"
)

var (
	// subdomainRe validates subdomain format
	subdomainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	// portRe validates port number format
	portRe = regexp.MustCompile(`^\d{1,5}$`)
)

// ValidateSubdomain checks if a subdomain string is valid
func ValidateSubdomain(s string) error {
	if !subdomainRe.MatchString(s) {
		return fmt.Errorf("invalid subdomain: use lowercase letters, digits, and hyphens (must start/end with alphanumeric)")
	}
	return nil
}

// ValidatePort checks if a port string is valid
func ValidatePort(p string) error {
	if !portRe.MatchString(p) {
		return fmt.Errorf("invalid port: must be a number between 1-65535")
	}
	return nil
}

// ValidateServiceType checks if a service type is valid
func ValidateServiceType(t string) error {
	for _, valid := range ValidServiceTypes {
		if t == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid service type %q: must be one of %v", t, ValidServiceTypes)
}

// ValidateAccessLevel checks if an access level is valid
func ValidateAccessLevel(level string) error {
	for _, valid := range ValidAccessLevels {
		if level == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid access level %q: must be one of %v", level, ValidAccessLevels)
}

// CURRENTLY DISABLED FOR BETTER DESIGN PRACTICES
// EnsurePortListening checks if a service is listening on the given port
// func EnsurePortListening(port string) error {
// 	addr := "127.0.0.1:" + port
// 	conn, err := net.DialTimeout("tcp", addr, 800*time.Millisecond)
// 	if err != nil {
// 		return fmt.Errorf("nothing listening on %s - start your service first", addr)
// 	}
// 	conn.Close()
// 	return nil
// }
