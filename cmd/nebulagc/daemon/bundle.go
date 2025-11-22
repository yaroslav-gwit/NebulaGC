package daemon

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// RequiredBundleFiles lists the files that must exist in a valid config bundle.
var RequiredBundleFiles = []string{
	"config.yml",
	"ca.crt",
	"host.crt",
	"host.key",
}

// BundleManager handles config bundle operations: validation, extraction, and atomic replacement.
type BundleManager struct {
	// configDir is the target directory for config files
	configDir string
}

// NewBundleManager creates a new bundle manager.
func NewBundleManager(configDir string) *BundleManager {
	return &BundleManager{
		configDir: configDir,
	}
}

// ApplyBundle validates, extracts, and atomically replaces config files with the new bundle.
//
// Process:
// 1. Validate bundle format (tar.gz with required files)
// 2. Create temporary directory
// 3. Extract bundle to temporary directory
// 4. Atomically rename temporary directory to config directory
// 5. Clean up old directory
//
// Parameters:
//   - ctx: Context for cancellation
//   - data: Bundle data (tar.gz format)
//   - version: Config version number
//
// Returns:
//   - error: Nil on success, error on failure
func (bm *BundleManager) ApplyBundle(ctx context.Context, data []byte, version int64) error {
	// Validate bundle
	if err := bm.validateBundle(data); err != nil {
		return fmt.Errorf("bundle validation failed: %w", err)
	}

	// Create temporary extraction directory
	tempDir := fmt.Sprintf("%s.tmp.%d", bm.configDir, version)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Extract bundle to temp directory
	if err := bm.extractBundle(data, tempDir); err != nil {
		os.RemoveAll(tempDir) // Clean up on failure
		return fmt.Errorf("failed to extract bundle: %w", err)
	}

	// Verify extracted files
	if err := bm.verifyExtractedFiles(tempDir); err != nil {
		os.RemoveAll(tempDir) // Clean up on failure
		return fmt.Errorf("extracted files verification failed: %w", err)
	}

	// Atomic replacement: rename old directory, move new directory into place
	if err := bm.atomicReplace(tempDir); err != nil {
		os.RemoveAll(tempDir) // Clean up on failure
		return fmt.Errorf("atomic replacement failed: %w", err)
	}

	return nil
}

// validateBundle checks that the bundle is valid tar.gz and contains required files.
func (bm *BundleManager) validateBundle(data []byte) error {
	// Decompress gzip
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid gzip format: %w", err)
	}
	defer gzReader.Close()

	// Read tar archive
	tarReader := tar.NewReader(gzReader)

	// Track found files
	foundFiles := make(map[string]bool)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid tar format: %w", err)
		}

		// Only track regular files
		if header.Typeflag == tar.TypeReg {
			foundFiles[filepath.Base(header.Name)] = true
		}
	}

	// Verify all required files are present
	for _, required := range RequiredBundleFiles {
		if !foundFiles[required] {
			return fmt.Errorf("missing required file: %s", required)
		}
	}

	return nil
}

// extractBundle extracts the tar.gz bundle to the specified directory.
func (bm *BundleManager) extractBundle(data []byte, destDir string) error {
	// Decompress gzip
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("gzip decompression failed: %w", err)
	}
	defer gzReader.Close()

	// Read tar archive
	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read failed: %w", err)
		}

		// Construct target path (only extract to destDir, prevent path traversal)
		targetPath := filepath.Join(destDir, filepath.Base(header.Name))

		switch header.Typeflag {
		case tar.TypeReg:
			// Extract regular file
			if err := bm.extractFile(tarReader, targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to extract %s: %w", header.Name, err)
			}
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		}
	}

	return nil
}

// extractFile writes a single file from tar archive to disk.
func (bm *BundleManager) extractFile(reader io.Reader, path string, mode os.FileMode) error {
	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Create file
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, reader); err != nil {
		return err
	}

	return nil
}

// verifyExtractedFiles checks that all required files exist after extraction.
func (bm *BundleManager) verifyExtractedFiles(dir string) error {
	for _, required := range RequiredBundleFiles {
		path := filepath.Join(dir, required)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("required file missing after extraction: %s", required)
		}
	}
	return nil
}

// atomicReplace performs an atomic directory replacement.
// It renames the current config directory to a backup, then renames the new directory into place.
func (bm *BundleManager) atomicReplace(tempDir string) error {
	backupDir := bm.configDir + ".old"

	// Remove old backup if it exists
	if _, err := os.Stat(backupDir); err == nil {
		if err := os.RemoveAll(backupDir); err != nil {
			return fmt.Errorf("failed to remove old backup: %w", err)
		}
	}

	// If config dir exists, rename it to backup
	if _, err := os.Stat(bm.configDir); err == nil {
		if err := os.Rename(bm.configDir, backupDir); err != nil {
			return fmt.Errorf("failed to backup current config: %w", err)
		}
	}

	// Rename temp directory to config directory (atomic operation)
	if err := os.Rename(tempDir, bm.configDir); err != nil {
		// Try to restore backup on failure
		if _, statErr := os.Stat(backupDir); statErr == nil {
			os.Rename(backupDir, bm.configDir)
		}
		return fmt.Errorf("failed to rename temp to config dir: %w", err)
	}

	// Success - remove backup directory
	if _, err := os.Stat(backupDir); err == nil {
		go os.RemoveAll(backupDir) // Async cleanup
	}

	return nil
}
