package util

import (
	"testing"
)

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "valid UUID v4",
			id:      "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "valid UUID v1",
			id:      "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			wantErr: false,
		},
		{
			name:    "invalid UUID - too short",
			id:      "550e8400-e29b-41d4",
			wantErr: true,
		},
		{
			name:    "invalid UUID - not hex",
			id:      "550e8400-e29b-41d4-a716-gggggggggggg",
			wantErr: true,
		},
		{
			name:    "empty string",
			id:      "",
			wantErr: true,
		},
		{
			name:    "random string",
			id:      "not-a-uuid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUUID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUUID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCIDR(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		wantErr bool
	}{
		{
			name:    "valid IPv4 CIDR",
			cidr:    "10.0.0.0/8",
			wantErr: false,
		},
		{
			name:    "valid IPv4 CIDR /24",
			cidr:    "192.168.1.0/24",
			wantErr: false,
		},
		{
			name:    "valid IPv4 CIDR /32",
			cidr:    "192.168.1.1/32",
			wantErr: false,
		},
		{
			name:    "valid IPv6 CIDR",
			cidr:    "2001:db8::/32",
			wantErr: false,
		},
		{
			name:    "invalid CIDR - no prefix",
			cidr:    "10.0.0.0",
			wantErr: true,
		},
		{
			name:    "invalid CIDR - bad prefix",
			cidr:    "10.0.0.0/33",
			wantErr: true,
		},
		{
			name:    "invalid CIDR - not IP",
			cidr:    "not-an-ip/8",
			wantErr: true,
		},
		{
			name:    "empty string",
			cidr:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCIDR(tt.cidr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCIDR() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{
			name:    "valid IPv4",
			ip:      "192.168.1.1",
			wantErr: false,
		},
		{
			name:    "valid IPv4 loopback",
			ip:      "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "valid IPv6",
			ip:      "2001:db8::1",
			wantErr: false,
		},
		{
			name:    "valid IPv6 loopback",
			ip:      "::1",
			wantErr: false,
		},
		{
			name:    "invalid IP - too many octets",
			ip:      "192.168.1.1.1",
			wantErr: true,
		},
		{
			name:    "invalid IP - out of range",
			ip:      "256.256.256.256",
			wantErr: true,
		},
		{
			name:    "invalid IP - not numeric",
			ip:      "not-an-ip",
			wantErr: true,
		},
		{
			name:    "empty string",
			ip:      "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIP(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateIPv4(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{
			name:    "valid IPv4",
			ip:      "192.168.1.1",
			wantErr: false,
		},
		{
			name:    "valid IPv4 loopback",
			ip:      "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "IPv6 should fail",
			ip:      "2001:db8::1",
			wantErr: true,
		},
		{
			name:    "IPv6 loopback should fail",
			ip:      "::1",
			wantErr: true,
		},
		{
			name:    "invalid IP",
			ip:      "not-an-ip",
			wantErr: true,
		},
		{
			name:    "empty string",
			ip:      "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIPv4(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIPv4() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name      string
		ip        string
		isPrivate bool
	}{
		{
			name:      "public IPv4",
			ip:        "8.8.8.8",
			isPrivate: false,
		},
		{
			name:      "private 10.0.0.0/8",
			ip:        "10.1.2.3",
			isPrivate: true,
		},
		{
			name:      "private 172.16.0.0/12",
			ip:        "172.16.0.1",
			isPrivate: true,
		},
		{
			name:      "private 192.168.0.0/16",
			ip:        "192.168.1.1",
			isPrivate: true,
		},
		{
			name:      "loopback",
			ip:        "127.0.0.1",
			isPrivate: true,
		},
		{
			name:      "link-local",
			ip:        "169.254.1.1",
			isPrivate: true,
		},
		{
			name:      "public IPv6",
			ip:        "2001:4860:4860::8888",
			isPrivate: false,
		},
		{
			name:      "IPv6 loopback",
			ip:        "::1",
			isPrivate: true,
		},
		{
			name:      "IPv6 link-local",
			ip:        "fe80::1",
			isPrivate: true,
		},
		{
			name:      "IPv6 unique local",
			ip:        "fc00::1",
			isPrivate: true,
		},
		{
			name:      "invalid IP",
			ip:        "not-an-ip",
			isPrivate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPrivateIP(tt.ip)
			if result != tt.isPrivate {
				t.Errorf("IsPrivateIP() = %v, want %v", result, tt.isPrivate)
			}
		})
	}
}

func TestValidatePortRange(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{
			name:    "valid port 80",
			port:    80,
			wantErr: false,
		},
		{
			name:    "valid port 443",
			port:    443,
			wantErr: false,
		},
		{
			name:    "valid port 65535",
			port:    65535,
			wantErr: false,
		},
		{
			name:    "invalid port 0",
			port:    0,
			wantErr: true,
		},
		{
			name:    "invalid port -1",
			port:    -1,
			wantErr: true,
		},
		{
			name:    "invalid port 65536",
			port:    65536,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePortRange(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePortRange() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMTU(t *testing.T) {
	tests := []struct {
		name    string
		mtu     int
		wantErr bool
	}{
		{
			name:    "valid MTU 1300",
			mtu:     1300,
			wantErr: false,
		},
		{
			name:    "valid MTU 1280 (minimum)",
			mtu:     1280,
			wantErr: false,
		},
		{
			name:    "valid MTU 9000 (maximum)",
			mtu:     9000,
			wantErr: false,
		},
		{
			name:    "valid MTU 1500",
			mtu:     1500,
			wantErr: false,
		},
		{
			name:    "invalid MTU 1279 (too low)",
			mtu:     1279,
			wantErr: true,
		},
		{
			name:    "invalid MTU 9001 (too high)",
			mtu:     9001,
			wantErr: true,
		},
		{
			name:    "invalid MTU 0",
			mtu:     0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMTU(tt.mtu)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMTU() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
