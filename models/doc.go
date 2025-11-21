// Package models provides shared data structures for the NebulaGC project.
//
// This package contains all core data models used across the control plane server,
// client SDK, and daemon. By keeping models in a separate package, they can be
// imported and reused by any component without creating circular dependencies.
//
// The models in this package represent:
//   - Tenants: Organizations that own clusters
//   - Clusters: Logical Nebula environments within a tenant
//   - Nodes: Individual machines enrolled in a cluster
//   - ConfigBundles: Versioned configuration archives distributed to nodes
//   - Replicas: Control plane instance registry for high availability
//   - Topology: Network topology information (routes, lighthouses, relays)
//
// All structs include JSON tags for API serialization and documentation comments
// explaining the purpose and constraints of each field.
package models
