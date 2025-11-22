// Package bundle provides utilities for validating and managing Nebula config bundles.
//
// Config bundles are tar.gz archives containing all files needed to run a Nebula node:
// - config.yml: Nebula configuration file
// - ca.crt: Certificate Authority certificate
// - crl.pem: Certificate Revocation List
// - host.crt: Node's host certificate
// - host.key: Node's private key
package bundle

import "errors"

const (
	// MaxBundleSize is the maximum allowed bundle size (10 MiB).
	MaxBundleSize = 10 * 1024 * 1024

	// RequiredFileConfig is the Nebula config file name.
	RequiredFileConfig = "config.yml"

	// RequiredFileCACert is the CA certificate file name.
	RequiredFileCACert = "ca.crt"

	// RequiredFileCRL is the certificate revocation list file name.
	RequiredFileCRL = "crl.pem"

	// RequiredFileHostCert is the host certificate file name.
	RequiredFileHostCert = "host.crt"

	// RequiredFileHostKey is the host private key file name.
	RequiredFileHostKey = "host.key"
)

// RequiredFiles is the list of all required files in a bundle.
var RequiredFiles = []string{
	RequiredFileConfig,
	RequiredFileCACert,
	RequiredFileCRL,
	RequiredFileHostCert,
	RequiredFileHostKey,
}

// Common bundle validation errors.
var (
	// ErrBundleTooLarge indicates the bundle exceeds the size limit.
	ErrBundleTooLarge = errors.New("bundle exceeds 10 MiB size limit")

	// ErrInvalidFormat indicates the bundle is not a valid gzip tar archive.
	ErrInvalidFormat = errors.New("bundle is not a valid gzip tar archive")

	// ErrMissingRequiredFile indicates a required file is missing from the bundle.
	ErrMissingRequiredFile = errors.New("bundle is missing required file")

	// ErrInvalidYAML indicates the config.yml file contains invalid YAML.
	ErrInvalidYAML = errors.New("config.yml contains invalid YAML")

	// ErrEmptyBundle indicates the bundle contains no files.
	ErrEmptyBundle = errors.New("bundle contains no files")
)

// ValidationResult holds the result of bundle validation.
type ValidationResult struct {
	// Valid indicates if the bundle passed all validations.
	Valid bool

	// Error contains the validation error if Valid is false.
	Error error

	// Files is the list of files found in the bundle.
	Files []string

	// Size is the total uncompressed size of the bundle in bytes.
	Size int64
}
