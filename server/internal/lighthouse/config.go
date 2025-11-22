package lighthouse

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// NebulaConfig represents the structure of a Nebula configuration file.
type NebulaConfig struct {
	PKI        PKIConfig        `yaml:"pki"`
	Lighthouse LighthouseConfig `yaml:"lighthouse"`
	Listen     ListenConfig     `yaml:"listen"`
	Punchy     PunchyConfig     `yaml:"punchy"`
	Tun        TunConfig        `yaml:"tun"`
	Logging    LoggingConfig    `yaml:"logging"`
	Firewall   FirewallConfig   `yaml:"firewall"`
}

// PKIConfig holds PKI file paths.
type PKIConfig struct {
	CA   string `yaml:"ca"`
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
	CRL  string `yaml:"crl,omitempty"`
}

// LighthouseConfig holds lighthouse settings.
type LighthouseConfig struct {
	AmLighthouse bool `yaml:"am_lighthouse"`
	ServeDNS     bool `yaml:"serve_dns"`
	Interval     int  `yaml:"interval"`
}

// ListenConfig holds network listener settings.
type ListenConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// PunchyConfig holds NAT traversal settings.
type PunchyConfig struct {
	Punch   bool `yaml:"punch"`
	Respond bool `yaml:"respond"`
}

// TunConfig holds TUN device settings.
type TunConfig struct {
	Disabled bool   `yaml:"disabled"`
	Dev      string `yaml:"dev"`
	MTU      int    `yaml:"mtu"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// FirewallConfig holds firewall rules.
type FirewallConfig struct {
	Outbound []FirewallRule `yaml:"outbound"`
	Inbound  []FirewallRule `yaml:"inbound"`
}

// FirewallRule represents a single firewall rule.
type FirewallRule struct {
	Port  interface{} `yaml:"port"`  // Can be "any" or number
	Proto string      `yaml:"proto"` // "any", "tcp", "udp", "icmp"
	Host  string      `yaml:"host"`  // "any" or CIDR
}

// GenerateConfig creates a Nebula configuration for a lighthouse.
//
// Parameters:
//   - clusterConfig: Cluster configuration data
//   - basePath: Base directory for config files
//
// Returns:
//   - Nebula configuration struct
func GenerateConfig(clusterConfig *ClusterConfig, basePath string) *NebulaConfig {
	clusterDir := filepath.Join(basePath, clusterConfig.ClusterID)

	return &NebulaConfig{
		PKI: PKIConfig{
			CA:   filepath.Join(clusterDir, "ca.crt"),
			Cert: filepath.Join(clusterDir, "host.crt"),
			Key:  filepath.Join(clusterDir, "host.key"),
			CRL:  filepath.Join(clusterDir, "crl.pem"),
		},
		Lighthouse: LighthouseConfig{
			AmLighthouse: true,
			ServeDNS:     false,
			Interval:     60,
		},
		Listen: ListenConfig{
			Host: "0.0.0.0",
			Port: clusterConfig.LighthousePort,
		},
		Punchy: PunchyConfig{
			Punch:   true,
			Respond: true,
		},
		Tun: TunConfig{
			Disabled: false,
			Dev:      fmt.Sprintf("nebula-%s", clusterConfig.ClusterID[:8]),
			MTU:      1300,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Firewall: FirewallConfig{
			Outbound: []FirewallRule{
				{Port: "any", Proto: "any", Host: "any"},
			},
			Inbound: []FirewallRule{
				{Port: "any", Proto: "icmp", Host: "any"},
			},
		},
	}
}

// WriteConfigFiles writes all config and PKI files for a cluster.
//
// This function creates the cluster directory and writes:
// - config.yml
// - ca.crt
// - crl.pem
// - host.crt
// - host.key
//
// Parameters:
//   - clusterConfig: Cluster configuration data
//   - basePath: Base directory for config files
//
// Returns:
//   - Path to the config file
//   - Error if any operation fails
func WriteConfigFiles(clusterConfig *ClusterConfig, basePath string) (string, error) {
	clusterDir := filepath.Join(basePath, clusterConfig.ClusterID)

	// Create cluster directory
	if err := os.MkdirAll(clusterDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create cluster directory: %w", err)
	}

	// Write PKI files
	files := map[string]string{
		"ca.crt":   clusterConfig.CACert,
		"crl.pem":  clusterConfig.CRL,
		"host.crt": clusterConfig.HostCert,
		"host.key": clusterConfig.HostKey,
	}

	for filename, content := range files {
		path := filepath.Join(clusterDir, filename)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			return "", fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Generate and write Nebula config
	nebulaConfig := GenerateConfig(clusterConfig, basePath)
	configData, err := yaml.Marshal(nebulaConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(clusterDir, "config.yml")
	if err := os.WriteFile(configPath, configData, 0600); err != nil {
		return "", fmt.Errorf("failed to write config.yml: %w", err)
	}

	return configPath, nil
}

// RemoveConfigFiles removes all config files for a cluster.
//
// This function removes the entire cluster directory.
//
// Parameters:
//   - clusterID: Cluster UUID
//   - basePath: Base directory for config files
//
// Returns:
//   - Error if removal fails
func RemoveConfigFiles(clusterID, basePath string) error {
	clusterDir := filepath.Join(basePath, clusterID)
	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("failed to remove cluster directory: %w", err)
	}
	return nil
}
