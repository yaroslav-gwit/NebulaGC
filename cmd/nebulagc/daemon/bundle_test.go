package daemon

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBundleManager_ValidateBundle(t *testing.T) {
	bm := NewBundleManager("/tmp/test")

	t.Run("valid bundle", func(t *testing.T) {
		bundle := createTestBundle(t, RequiredBundleFiles)
		if err := bm.validateBundle(bundle); err != nil {
			t.Errorf("validateBundle() error = %v", err)
		}
	})

	t.Run("missing required file", func(t *testing.T) {
		bundle := createTestBundle(t, []string{"config.yml", "ca.crt"}) // Missing host.crt and host.key
		if err := bm.validateBundle(bundle); err == nil {
			t.Error("validateBundle() expected error for missing files")
		}
	})

	t.Run("invalid gzip", func(t *testing.T) {
		invalidData := []byte("not a gzip file")
		if err := bm.validateBundle(invalidData); err == nil {
			t.Error("validateBundle() expected error for invalid gzip")
		}
	})

	t.Run("invalid tar", func(t *testing.T) {
		// Create gzip with invalid tar content
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		gzWriter.Write([]byte("not a tar file"))
		gzWriter.Close()

		if err := bm.validateBundle(buf.Bytes()); err == nil {
			t.Error("validateBundle() expected error for invalid tar")
		}
	})
}

func TestBundleManager_ExtractBundle(t *testing.T) {
	tempDir := t.TempDir()

	bm := NewBundleManager(filepath.Join(tempDir, "config"))
	bundle := createTestBundle(t, RequiredBundleFiles)

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extract dir: %v", err)
	}

	if err := bm.extractBundle(bundle, extractDir); err != nil {
		t.Errorf("extractBundle() error = %v", err)
	}

	// Verify all files were extracted
	for _, filename := range RequiredBundleFiles {
		path := filepath.Join(extractDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not extracted", filename)
		}
	}
}

func TestBundleManager_VerifyExtractedFiles(t *testing.T) {
	tempDir := t.TempDir()

	bm := NewBundleManager(tempDir)

	t.Run("all files present", func(t *testing.T) {
		// Create all required files
		for _, filename := range RequiredBundleFiles {
			path := filepath.Join(tempDir, filename)
			if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
		}

		if err := bm.verifyExtractedFiles(tempDir); err != nil {
			t.Errorf("verifyExtractedFiles() error = %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		missingDir := filepath.Join(tempDir, "missing")
		if err := os.MkdirAll(missingDir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}

		// Only create some files
		os.WriteFile(filepath.Join(missingDir, "config.yml"), []byte("test"), 0644)

		if err := bm.verifyExtractedFiles(missingDir); err == nil {
			t.Error("verifyExtractedFiles() expected error for missing files")
		}
	})
}

func TestBundleManager_AtomicReplace(t *testing.T) {
	tempDir := t.TempDir()

	configDir := filepath.Join(tempDir, "config")
	newDir := filepath.Join(tempDir, "new")

	bm := NewBundleManager(configDir)

	// Create existing config directory with a file
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	os.WriteFile(filepath.Join(configDir, "old.txt"), []byte("old content"), 0644)

	// Create new directory with different content
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("Failed to create new dir: %v", err)
	}
	os.WriteFile(filepath.Join(newDir, "new.txt"), []byte("new content"), 0644)

	// Perform atomic replace
	if err := bm.atomicReplace(newDir); err != nil {
		t.Errorf("atomicReplace() error = %v", err)
	}

	// Verify new directory is now in place
	if _, err := os.Stat(filepath.Join(configDir, "new.txt")); os.IsNotExist(err) {
		t.Error("New file should exist in config directory")
	}

	// Verify old file is gone
	if _, err := os.Stat(filepath.Join(configDir, "old.txt")); err == nil {
		t.Error("Old file should not exist in config directory")
	}

	// Verify temp directory is gone
	if _, err := os.Stat(newDir); err == nil {
		t.Error("Temp directory should be removed")
	}
}

func TestBundleManager_ApplyBundle(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")

	bm := NewBundleManager(configDir)
	bundle := createTestBundle(t, RequiredBundleFiles)

	ctx := context.Background()
	if err := bm.ApplyBundle(ctx, bundle, 1); err != nil {
		t.Errorf("ApplyBundle() error = %v", err)
	}

	// Verify config directory was created
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error("Config directory should exist after ApplyBundle")
	}

	// Verify all files were extracted
	for _, filename := range RequiredBundleFiles {
		path := filepath.Join(configDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not found", filename)
		}
	}
}

func TestBundleManager_ApplyBundle_InvalidBundle(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")

	bm := NewBundleManager(configDir)

	// Try to apply invalid bundle
	invalidData := []byte("not a valid bundle")

	ctx := context.Background()
	if err := bm.ApplyBundle(ctx, invalidData, 1); err == nil {
		t.Error("ApplyBundle() expected error for invalid bundle")
	}

	// Verify no temp directories left behind
	entries, _ := os.ReadDir(tempDir)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Errorf("Temp directory %s should have been cleaned up", entry.Name())
		}
	}
}

// createTestBundle creates a valid tar.gz bundle with the specified files.
func createTestBundle(t *testing.T, files []string) []byte {
	var buf bytes.Buffer

	// Create gzip writer
	gzWriter := gzip.NewWriter(&buf)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add each file to the tar archive
	for _, filename := range files {
		content := []byte("test content for " + filename)

		header := &tar.Header{
			Name: filename,
			Mode: 0644,
			Size: int64(len(content)),
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}

		if _, err := tarWriter.Write(content); err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}
	}

	tarWriter.Close()
	gzWriter.Close()

	return buf.Bytes()
}
