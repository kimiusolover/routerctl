package firmwaregate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRequiresSupportedDeviceAndVerifiedPartitions(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "devices", "ax23v-v1")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "device.yaml"), []byte("id: ax23v-v1\nstatus: discovery\npartitions: partitions.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "partitions.yaml"), []byte("status: unverified\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Check(repo, "ax23v-v1"); err == nil {
		t.Fatal("Check accepted discovery device")
	}
	if err := os.WriteFile(filepath.Join(dir, "device.yaml"), []byte("id: ax23v-v1\nstatus: supported\npartitions: partitions.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "partitions.yaml"), []byte("status: verified\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Check(repo, "ax23v-v1"); err != nil {
		t.Fatal(err)
	}
}
