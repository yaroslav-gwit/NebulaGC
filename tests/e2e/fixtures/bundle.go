package fixtures

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

// ValidBundle creates a valid Nebula config bundle (tar.gz) for testing.
func ValidBundle(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer

	// Create gzip writer
	gzWriter := gzip.NewWriter(&buf)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add config.yml file
	configContent := `# Nebula Configuration
pki:
  ca: /etc/nebula/ca.crt
  cert: /etc/nebula/host.crt
  key: /etc/nebula/host.key

lighthouse:
  am_lighthouse: false
  interval: 60
  hosts:
    - "192.168.100.1"

listen:
  host: 0.0.0.0
  port: 4242

punchy:
  punch: true
  respond: true

tun:
  dev: nebula1
  drop_local_broadcast: false
  drop_multicast: false
  tx_queue: 500
  mtu: 1300

logging:
  level: info
  format: text

firewall:
  outbound:
    - port: any
      proto: any
      host: any

  inbound:
    - port: any
      proto: any
      host: any
`

	header := &tar.Header{
		Name: "config.yml",
		Mode: 0600,
		Size: int64(len(configContent)),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}

	if _, err := tarWriter.Write([]byte(configContent)); err != nil {
		t.Fatalf("failed to write tar content: %v", err)
	}

	// Add ca.crt file
	caContent := `-----BEGIN NEBULA CERTIFICATE-----
CjYKBm5lYnVsYRIgwBONWq3tCmJB/MqrMkZBn9RhpVfZn0qwEuKzDGqZmOISIDEw
MTAxMDEwMDAwMDAoAToTChdDQTpuZWJ1bGEtdGVzdC1yb290
-----END NEBULA CERTIFICATE-----
`

	header = &tar.Header{
		Name: "ca.crt",
		Mode: 0600,
		Size: int64(len(caContent)),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}

	if _, err := tarWriter.Write([]byte(caContent)); err != nil {
		t.Fatalf("failed to write tar content: %v", err)
	}

	// Close writers to flush
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}

	if err := gzWriter.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}

// InvalidBundle creates an invalid bundle (not tar.gz) for testing.
func InvalidBundle(t *testing.T) []byte {
	t.Helper()
	return []byte("this is not a valid tar.gz file")
}

// MissingConfigBundle creates a bundle without config.yml for testing.
func MissingConfigBundle(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer

	gzWriter := gzip.NewWriter(&buf)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add a different file (not config.yml)
	content := "some content"
	header := &tar.Header{
		Name: "other.txt",
		Mode: 0600,
		Size: int64(len(content)),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}

	if _, err := tarWriter.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write tar content: %v", err)
	}

	tarWriter.Close()
	gzWriter.Close()

	return buf.Bytes()
}

// OversizedBundle creates a bundle larger than allowed size for testing.
func OversizedBundle(t *testing.T, sizeMB int) []byte {
	t.Helper()

	var buf bytes.Buffer

	gzWriter := gzip.NewWriter(&buf)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Create large content
	largeContent := bytes.Repeat([]byte("x"), sizeMB*1024*1024)

	header := &tar.Header{
		Name: "config.yml",
		Mode: 0600,
		Size: int64(len(largeContent)),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}

	if _, err := tarWriter.Write(largeContent); err != nil {
		t.Fatalf("failed to write tar content: %v", err)
	}

	tarWriter.Close()
	gzWriter.Close()

	return buf.Bytes()
}

// BundleWithFiles creates a bundle with specified files.
func BundleWithFiles(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer

	gzWriter := gzip.NewWriter(&buf)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	for filename, content := range files {
		header := &tar.Header{
			Name: filename,
			Mode: 0600,
			Size: int64(len(content)),
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header for %s: %v", filename, err)
		}

		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write tar content for %s: %v", filename, err)
		}
	}

	tarWriter.Close()
	gzWriter.Close()

	return buf.Bytes()
}
