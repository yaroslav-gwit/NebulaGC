package models

import "time"

// Tenant represents an organization that owns one or more Nebula clusters.
// Tenants provide isolation between different customers or organizational units.
// Each tenant can contain multiple clusters (e.g., production, staging, dev).
type Tenant struct {
	// ID is the unique identifier for this tenant (UUID v4 format)
	ID string `json:"id" db:"id"`

	// Name is the human-readable tenant name (e.g., "Acme Corporation")
	// Maximum length: 255 characters
	Name string `json:"name" db:"name"`

	// CreatedAt is the timestamp when this tenant was created
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TenantCreateRequest represents the request body for creating a new tenant.
type TenantCreateRequest struct {
	// Name is the desired tenant name (required)
	// Must be 1-255 characters
	Name string `json:"name" binding:"required,min=1,max=255"`
}

// TenantListResponse represents the response for listing tenants.
type TenantListResponse struct {
	// Tenants is the list of all tenants in the system
	Tenants []Tenant `json:"tenants"`

	// Total is the total number of tenants
	Total int `json:"total"`
}
