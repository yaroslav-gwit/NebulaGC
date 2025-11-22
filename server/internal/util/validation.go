package util

import (
	"fmt"
	"net"

	"github.com/google/uuid"
)

// ValidateUUID checks if a string is a valid UUID v4 format.
//
// Parameters:
//   - id: The string to validate as UUID
//
// Returns:
//   - error: An error if the string is not a valid UUID, nil otherwise
//
// Example:
//
//	if err := util.ValidateUUID(nodeID); err != nil {
//	    return models.ErrInvalidRequest
//	}
func ValidateUUID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("invalid UUID format: %w", err)
	}
	return nil
}

// ValidateCIDR checks if a string is valid CIDR notation (e.g., "10.0.0.0/8").
//
// Parameters:
//   - cidr: The string to validate as CIDR notation
//
// Returns:
//   - error: An error if the string is not valid CIDR, nil otherwise
//
// Example:
//
//	for _, route := range routes {
//	    if err := util.ValidateCIDR(route); err != nil {
//	        return fmt.Errorf("invalid route %q: %w", route, err)
//	    }
//	}
func ValidateCIDR(cidr string) error {
	if _, _, err := net.ParseCIDR(cidr); err != nil {
		return fmt.Errorf("invalid CIDR notation: %w", err)
	}
	return nil
}

// ValidateIP checks if a string is a valid IP address (IPv4 or IPv6).
//
// Parameters:
//   - ip: The string to validate as IP address
//
// Returns:
//   - error: An error if the string is not a valid IP, nil otherwise
//
// Example:
//
//	if err := util.ValidateIP(lighthouseIP); err != nil {
//	    return models.ErrInvalidRequest
//	}
func ValidateIP(ip string) error {
	if parsed := net.ParseIP(ip); parsed == nil {
		return fmt.Errorf("invalid IP address")
	}
	return nil
}

// ValidateIPv4 checks if a string is specifically a valid IPv4 address.
//
// Parameters:
//   - ip: The string to validate as IPv4 address
//
// Returns:
//   - error: An error if the string is not a valid IPv4, nil otherwise
//
// Example:
//
//	if err := util.ValidateIPv4(lighthousePublicIP); err != nil {
//	    return fmt.Errorf("lighthouse requires IPv4 address: %w", err)
//	}
func ValidateIPv4(ip string) error {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP address")
	}
	if parsed.To4() == nil {
		return fmt.Errorf("not an IPv4 address")
	}
	return nil
}

// IsPrivateIP checks if an IP address is in a private range.
// This is useful for SSRF protection.
//
// Private ranges:
// - 10.0.0.0/8 (RFC 1918)
// - 172.16.0.0/12 (RFC 1918)
// - 192.168.0.0/16 (RFC 1918)
// - 127.0.0.0/8 (loopback)
// - 169.254.0.0/16 (link-local)
// - ::1/128 (IPv6 loopback)
// - fe80::/10 (IPv6 link-local)
// - fc00::/7 (IPv6 unique local)
//
// Parameters:
//   - ip: The IP address to check
//
// Returns:
//   - bool: true if the IP is in a private range, false otherwise
//
// Example:
//
//	if util.IsPrivateIP(remoteIP) {
//	    return errors.New("private IP addresses not allowed")
//	}
func IsPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	// Check special addresses
	if parsed.IsLoopback() || parsed.IsLinkLocalUnicast() || parsed.IsLinkLocalMulticast() {
		return true
	}

	// Private IPv4 ranges
	privateIPv4Ranges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16", // Link-local
	}

	for _, cidr := range privateIPv4Ranges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(parsed) {
			return true
		}
	}

	// Private IPv6 ranges
	privateIPv6Ranges := []string{
		"fc00::/7",  // Unique local
		"fe80::/10", // Link-local
	}

	for _, cidr := range privateIPv6Ranges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(parsed) {
			return true
		}
	}

	return false
}

// ValidatePortRange checks if a port number is in valid range (1-65535).
//
// Parameters:
//   - port: The port number to validate
//
// Returns:
//   - error: An error if the port is out of range, nil otherwise
//
// Example:
//
//	if err := util.ValidatePortRange(lighthousePort); err != nil {
//	    return models.ErrInvalidRequest
//	}
func ValidatePortRange(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}
	return nil
}

// ValidateMTU checks if an MTU value is in valid range for Nebula.
//
// Parameters:
//   - mtu: The MTU value to validate
//
// Returns:
//   - error: An error if the MTU is out of range, nil otherwise
//
// Valid range: 1280-9000 bytes
// - 1280 is the IPv6 minimum MTU
// - 9000 is a common jumbo frame size
//
// Example:
//
//	if err := util.ValidateMTU(req.MTU); err != nil {
//	    return models.ErrInvalidRequest
//	}
func ValidateMTU(mtu int) error {
	const (
		MinMTU = 1280 // IPv6 minimum
		MaxMTU = 9000 // Jumbo frames
	)

	if mtu < MinMTU || mtu > MaxMTU {
		return fmt.Errorf("MTU must be between %d and %d, got %d", MinMTU, MaxMTU, mtu)
	}
	return nil
}
