package bundle

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// Validate checks if the bundle meets all requirements.
//
// This function validates:
// - Bundle size (must be <= 10 MiB)
// - Archive format (must be valid gzip tar)
// - Required files presence
// - YAML syntax in config.yml
//
// Parameters:
//   - data: The bundle data as bytes
//
// Returns:
//   - *ValidationResult: Validation result with details
func Validate(data []byte) *ValidationResult {
	// Check size
	if len(data) > MaxBundleSize {
		return &ValidationResult{
			Valid: false,
			Error: ErrBundleTooLarge,
			Size:  int64(len(data)),
		}
	}

	// Try to open as gzip
	gzReader, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Error: fmt.Errorf("%w: %v", ErrInvalidFormat, err),
			Size:  int64(len(data)),
		}
	}
	defer gzReader.Close()

	// Try to read as tar
	tarReader := tar.NewReader(gzReader)

	// Track files found
	filesFound := make(map[string]bool)
	var configYAML []byte
	var totalSize int64

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return &ValidationResult{
				Valid: false,
				Error: fmt.Errorf("%w: %v", ErrInvalidFormat, err),
				Size:  totalSize,
			}
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Track file
		fileName := header.Name
		filesFound[fileName] = true
		totalSize += header.Size

		// If this is config.yml, read it for YAML validation
		if fileName == RequiredFileConfig {
			configYAML, err = io.ReadAll(tarReader)
			if err != nil {
				return &ValidationResult{
					Valid: false,
					Error: fmt.Errorf("failed to read config.yml: %w", err),
					Size:  totalSize,
				}
			}
		}
	}

	// Check if bundle is empty
	if len(filesFound) == 0 {
		return &ValidationResult{
			Valid: false,
			Error: ErrEmptyBundle,
			Size:  totalSize,
		}
	}

	// Check all required files are present
	for _, required := range RequiredFiles {
		if !filesFound[required] {
			return &ValidationResult{
				Valid: false,
				Error: fmt.Errorf("%w: %s", ErrMissingRequiredFile, required),
				Size:  totalSize,
			}
		}
	}

	// Validate config.yml YAML syntax
	if len(configYAML) > 0 {
		var config interface{}
		if err := yaml.Unmarshal(configYAML, &config); err != nil {
			return &ValidationResult{
				Valid: false,
				Error: fmt.Errorf("%w: %v", ErrInvalidYAML, err),
				Size:  totalSize,
			}
		}
	}

	// Build file list
	fileList := make([]string, 0, len(filesFound))
	for file := range filesFound{
		fileList = append(fileList, file)
	}

	return &ValidationResult{
		Valid: true,
		Files: fileList,
		Size:  totalSize,
	}
}
