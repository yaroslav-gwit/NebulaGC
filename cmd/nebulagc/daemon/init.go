package daemon

import (
	"fmt"
	"time"

	"github.com/yaroslav/nebulagc/sdk"
)

// Daemon represents the main daemon instance managing multiple Nebula clusters.
type Daemon struct {
	// Config is the daemon configuration.
	Config *DaemonConfig

	// Clients maps cluster names to their SDK clients.
	Clients map[string]*sdk.Client
}

// Initialize creates and initializes a new daemon instance from configuration.
//
// This function:
//  1. Loads the daemon configuration
//  2. Creates SDK clients for each cluster
//  3. Validates connectivity to control plane
//
// Parameters:
//   - configPath: Optional path to config file (uses default locations if empty)
//
// Returns:
//   - *Daemon: Initialized daemon instance ready to start managing clusters
//   - error: Initialization error
func Initialize(configPath string) (*Daemon, error) {
	// Load configuration
	var config *DaemonConfig
	var err error

	if configPath != "" {
		config, err = LoadConfigFromPath(configPath)
	} else {
		config, err = LoadConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create daemon instance
	daemon := &Daemon{
		Config:  config,
		Clients: make(map[string]*sdk.Client),
	}

	// Initialize SDK client for each cluster
	for _, clusterConfig := range config.Clusters {
		client, err := createSDKClient(config.ControlPlaneURLs, clusterConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create SDK client for cluster %s: %w", clusterConfig.Name, err)
		}

		daemon.Clients[clusterConfig.Name] = client
	}

	return daemon, nil
}

// createSDKClient creates and configures an SDK client for a cluster.
func createSDKClient(controlPlaneURLs []string, clusterConfig ClusterConfig) (*sdk.Client, error) {
	// Build SDK configuration
	sdkConfig := sdk.ClientConfig{
		BaseURLs:      controlPlaneURLs,
		TenantID:      clusterConfig.TenantID,
		ClusterID:     clusterConfig.ClusterID,
		NodeID:        clusterConfig.NodeID,
		NodeToken:     clusterConfig.NodeToken,
		ClusterToken:  clusterConfig.ClusterToken,
		RetryAttempts: 3,
		RetryWaitMin:  1 * time.Second,
		RetryWaitMax:  10 * time.Second,
	}

	// Create client
	client, err := sdk.NewClient(sdkConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create SDK client: %w", err)
	}

	return client, nil
}

// GetClient returns the SDK client for a specific cluster.
//
// Parameters:
//   - clusterName: Name of the cluster (from config)
//
// Returns:
//   - *sdk.Client: SDK client for the cluster
//   - error: Error if cluster not found
func (d *Daemon) GetClient(clusterName string) (*sdk.Client, error) {
	client, ok := d.Clients[clusterName]
	if !ok {
		return nil, fmt.Errorf("cluster %s not found", clusterName)
	}
	return client, nil
}

// GetClusterConfig returns the configuration for a specific cluster.
//
// Parameters:
//   - clusterName: Name of the cluster (from config)
//
// Returns:
//   - *ClusterConfig: Cluster configuration
//   - error: Error if cluster not found
func (d *Daemon) GetClusterConfig(clusterName string) (*ClusterConfig, error) {
	for i := range d.Config.Clusters {
		if d.Config.Clusters[i].Name == clusterName {
			return &d.Config.Clusters[i], nil
		}
	}
	return nil, fmt.Errorf("cluster %s not found", clusterName)
}

// ClusterNames returns the list of all configured cluster names.
func (d *Daemon) ClusterNames() []string {
	names := make([]string, len(d.Config.Clusters))
	for i, cluster := range d.Config.Clusters {
		names[i] = cluster.Name
	}
	return names
}
