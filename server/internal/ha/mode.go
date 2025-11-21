// Package ha defines high availability primitives for NebulaGC.
package ha

// Mode represents the runtime role of a control plane instance.
type Mode string

const (
	// ModeMaster indicates this instance accepts write operations.
	ModeMaster Mode = "master"

	// ModeReplica indicates this instance is read-only and should forward writes to the master.
	ModeReplica Mode = "replica"
)

// ValidateMode ensures the provided mode is one of the supported values.
func ValidateMode(mode Mode) bool {
	return mode == ModeMaster || mode == ModeReplica
}
