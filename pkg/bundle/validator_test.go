package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"testing"
)

// createTestBundle creates a tar.gz bundle with the specified files.
func createTestBundle(files map[string]string) []byte {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			panic(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			panic(err)
		}
	}

	tw.Close()
	gzw.Close()

	return buf.Bytes()
}

func TestValidate_ValidBundle(t *testing.T) {
	validYAML := `pki:
  ca: /etc/nebula/ca.crt
  cert: /etc/nebula/host.crt
  key: /etc/nebula/host.key
`

	bundle := createTestBundle(map[string]string{
		RequiredFileConfig:   validYAML,
		RequiredFileCACert:   "-----BEGIN CERTIFICATE-----\nca cert\n-----END CERTIFICATE-----",
		RequiredFileCRL:      "-----BEGIN X509 CRL-----\ncrl\n-----END X509 CRL-----",
		RequiredFileHostCert: "-----BEGIN CERTIFICATE-----\nhost cert\n-----END CERTIFICATE-----",
		RequiredFileHostKey:  "-----BEGIN NEBULA PRIVATE KEY-----\nkey\n-----END NEBULA PRIVATE KEY-----",
	})

	result := Validate(bundle)

	if !result.Valid {
		t.Errorf("Expected valid bundle, got error: %v", result.Error)
	}

	if len(result.Files) != 5 {
		t.Errorf("Expected 5 files, got %d", len(result.Files))
	}

	if result.Size == 0 {
		t.Error("Expected non-zero size")
	}
}

func TestValidate_BundleTooLarge(t *testing.T) {
	// Create bundle larger than MaxBundleSize
	largeData := make([]byte, MaxBundleSize+1)

	result := Validate(largeData)

	if result.Valid {
		t.Error("Expected invalid bundle due to size")
	}

	if result.Error != ErrBundleTooLarge {
		t.Errorf("Expected ErrBundleTooLarge, got %v", result.Error)
	}
}

func TestValidate_InvalidGzip(t *testing.T) {
	invalidData := []byte("not a gzip file")

	result := Validate(invalidData)

	if result.Valid {
		t.Error("Expected invalid bundle due to format")
	}

	if !errors.Is(result.Error, ErrInvalidFormat) {
		t.Errorf("Expected ErrInvalidFormat, got %v", result.Error)
	}
}

func TestValidate_EmptyBundle(t *testing.T) {
	bundle := createTestBundle(map[string]string{})

	result := Validate(bundle)

	if result.Valid {
		t.Error("Expected invalid bundle due to empty content")
	}

	if result.Error != ErrEmptyBundle {
		t.Errorf("Expected ErrEmptyBundle, got %v", result.Error)
	}
}

func TestValidate_MissingRequiredFile(t *testing.T) {
	validYAML := `pki:
  ca: /etc/nebula/ca.crt
`

	// Missing host.key
	bundle := createTestBundle(map[string]string{
		RequiredFileConfig:   validYAML,
		RequiredFileCACert:   "ca cert",
		RequiredFileCRL:      "crl",
		RequiredFileHostCert: "host cert",
		// Missing RequiredFileHostKey
	})

	result := Validate(bundle)

	if result.Valid {
		t.Error("Expected invalid bundle due to missing file")
	}

	if !errors.Is(result.Error, ErrMissingRequiredFile) {
		t.Errorf("Expected ErrMissingRequiredFile, got %v", result.Error)
	}
}

func TestValidate_InvalidYAML(t *testing.T) {
	invalidYAML := `this is not valid yaml: [[[`

	bundle := createTestBundle(map[string]string{
		RequiredFileConfig:   invalidYAML,
		RequiredFileCACert:   "ca cert",
		RequiredFileCRL:      "crl",
		RequiredFileHostCert: "host cert",
		RequiredFileHostKey:  "key",
	})

	result := Validate(bundle)

	if result.Valid {
		t.Error("Expected invalid bundle due to YAML syntax")
	}

	if !errors.Is(result.Error, ErrInvalidYAML) {
		t.Errorf("Expected ErrInvalidYAML, got %v", result.Error)
	}
}

func TestValidate_ValidBundleWithExtraFiles(t *testing.T) {
	validYAML := `pki:
  ca: /etc/nebula/ca.crt
`

	// Bundle with extra files (should still be valid)
	bundle := createTestBundle(map[string]string{
		RequiredFileConfig:   validYAML,
		RequiredFileCACert:   "ca cert",
		RequiredFileCRL:      "crl",
		RequiredFileHostCert: "host cert",
		RequiredFileHostKey:  "key",
		"extra.txt":          "extra file",
		"scripts/init.sh":    "#!/bin/bash\necho hello",
	})

	result := Validate(bundle)

	if !result.Valid {
		t.Errorf("Expected valid bundle with extra files, got error: %v", result.Error)
	}

	if len(result.Files) != 7 {
		t.Errorf("Expected 7 files, got %d", len(result.Files))
	}
}

func TestValidate_InvalidTarArchive(t *testing.T) {
	// Create valid gzip but invalid tar
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	gzw.Write([]byte("not a tar archive"))
	gzw.Close()

	result := Validate(buf.Bytes())

	if result.Valid {
		t.Error("Expected invalid bundle due to tar format")
	}

	if !errors.Is(result.Error, ErrInvalidFormat) {
		t.Errorf("Expected ErrInvalidFormat, got %v", result.Error)
	}
}
