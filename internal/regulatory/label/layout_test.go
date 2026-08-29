package label

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLayoutAcceptsOnlyEvidenceRoles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.yaml")
	data := `apiVersion: routerctl.regulatory/v1
kind: LabelLayout
metadata:
  name: tp-link/archer-ax23v/jp-1.0
device:
  vendor: TP-Link
  model: Archer AX23V
  revision: JP/1.0
crops:
  device_identity_crop:
    geometry: 110x43+10+19
  giteki_mark_and_number_crop:
    geometry: 85x30+12+62
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	layout, crops, err := LoadLayout(path)
	if err != nil {
		t.Fatal(err)
	}
	if layout.Metadata.Name != "tp-link/archer-ax23v/jp-1.0" || len(crops) != 2 {
		t.Fatalf("layout = %#v, crops = %#v", layout, crops)
	}
}

func TestLoadLayoutRejectsSensitiveCrop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.yaml")
	data := `apiVersion: routerctl.regulatory/v1
kind: LabelLayout
metadata: {name: tp-link/archer-ax23v/jp-1.0}
crops:
  device_identity_crop: {geometry: 110x43+10+19}
  wifi_password_crop: {geometry: 85x30+12+62}
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadLayout(path); err == nil {
		t.Fatal("LoadLayout accepted sensitive crop")
	}
}
