package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindDeviceManifests(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "devices", "test.yaml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(`apiVersion: routerctl.dev/v1alpha1
kind: Device
metadata: {name: test-router}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "other.yaml"), []byte(`apiVersion: routerctl.dev/device/v1alpha1
kind: HardwareDevice
metadata: {name: not-a-manifest}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	ignoredPath := filepath.Join(root, "packaging", "arch", "device.yaml")
	if err := os.MkdirAll(filepath.Dir(ignoredPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignoredPath, []byte(`apiVersion: routerctl.dev/v1alpha1
kind: Device
metadata: {name: packaged-copy}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	candidates := findDeviceManifests(root)
	if len(candidates) != 1 {
		t.Fatalf("findDeviceManifests() found %d candidates, want 1: %#v", len(candidates), candidates)
	}
	if candidates[0].Path != filepath.Join("devices", "test.yaml") || candidates[0].Name != "test-router" {
		t.Fatalf("unexpected candidate: %#v", candidates[0])
	}
}

func TestNextStepsShowsCandidatesAndNextCommand(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "example.yaml")
	if err := os.WriteFile(manifestPath, []byte(`apiVersion: routerctl.dev/v1alpha1
kind: Device
metadata: {name: test-router}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	nextStepsAt(&output, root)
	if !strings.Contains(output.String(), "routerctl: start by checking a device manifest.") {
		t.Fatalf("nextSteps() did not provide an entry point: %s", output.String())
	}
	if !strings.Contains(output.String(), "example.yaml (test-router)") || !strings.Contains(output.String(), "routerctl verify example.yaml") {
		t.Fatalf("nextSteps() did not suggest verification: %s", output.String())
	}
}

func TestResolveRejectsGitHubManifestWithoutRepository(t *testing.T) {
	path := filepath.Join(t.TempDir(), "device.yaml")
	data := "apiVersion: routerctl.dev/v1alpha1\nkind: Device\nmetadata:\n  name: test\nspec:\n  device: ax23v-v1\n  backend: github\n  transport: ssh\n  target: ax23v-v1.bin\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"resolve", path}, BuildInfo{}); err == nil {
		t.Fatal("Run(resolve) succeeded without github.repository")
	}
}

func TestBuildRejectsUnsafeFirmwareBeforeRunningCommand(t *testing.T) {
	root := t.TempDir()
	firmware := filepath.Join(root, "router-firmware")
	deviceDir := filepath.Join(firmware, "devices", "ax23v-v1")
	if err := os.MkdirAll(deviceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deviceDir, "device.yaml"), []byte("id: ax23v-v1\nstatus: discovery\npartitions: partitions.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deviceDir, "partitions.yaml"), []byte("status: unverified\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "build.yaml")
	data := `apiVersion: routerctl.dev/v1alpha1
kind: Device
metadata: {name: ax23v}
spec:
  device: ax23v-v1
  backend: native
  transport: ssh
  build:
    profile: ax23v-v1
    repository: router-firmware
    command: [sh, -c, "exit 99"]
    output: dist/ax23v-v1.bin
  artifact:
    expected: {device: ax23v-v1, format: tplink-safeloader, max_size_bytes: 16515072}
`
	if err := os.WriteFile(manifestPath, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"build", manifestPath}, BuildInfo{}); err == nil {
		t.Fatal("build accepted an unsafe firmware target")
	}
}

func TestRegulatoryLabelExtract_CLI(t *testing.T) {
	tempDir := t.TempDir()

	// Create synthetic image
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	imgPath := filepath.Join(tempDir, "label.png")
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	imgBytes, _ := os.ReadFile(imgPath)
	imgHash := sha256.Sum256(imgBytes)
	validSHA := hex.EncodeToString(imgHash[:])
	layoutPath := filepath.Join(tempDir, "layout.yaml")
	layout := `apiVersion: routerctl.regulatory/v1
kind: LabelLayout
metadata:
  name: tp-link/archer-ax23v/jp-1.0
crops:
  device_identity_crop:
    geometry: 200x50+10+10
  giteki_mark_and_number_crop:
    geometry: 200x80+10+70
`
	if err := os.WriteFile(layoutPath, []byte(layout), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("MissingRequiredFlags", func(t *testing.T) {
		err := Run([]string{"regulatory", "label", "extract"}, BuildInfo{})
		if err == nil {
			t.Fatal("expected error for missing flags, got nil")
		}
	})

	t.Run("SourceSHA256Mismatch", func(t *testing.T) {
		outDir := filepath.Join(tempDir, "bundle_mismatch")
		err := Run([]string{
			"regulatory", "label", "extract",
			"--image", imgPath,
			"--source-sha256", "0000000000000000000000000000000000000000000000000000000000000000",
			"--layout", layoutPath,
			"--out", outDir,
		}, BuildInfo{})
		if err == nil {
			t.Fatal("expected error for mismatched source-sha256, got nil")
		}
	})

	t.Run("LayoutIsRequired", func(t *testing.T) {
		outDir := filepath.Join(tempDir, "bundle_missing_crop")
		err := Run([]string{
			"regulatory", "label", "extract",
			"--image", imgPath,
			"--out", outDir,
		}, BuildInfo{})
		if err == nil {
			t.Fatal("expected error for missing mandatory crop, got nil")
		}
	})

	t.Run("SuccessfulExtractionAndVerifyCLI", func(t *testing.T) {
		outDir := filepath.Join(tempDir, "bundle_out")
		err := Run([]string{
			"regulatory", "label", "extract",
			"--image", imgPath,
			"--source-sha256", validSHA,
			"--layout", layoutPath,
			"--out", outDir,
			"--vendor", "TP-Link",
			"--model", "Archer AX23V",
			"--revision", "JP/1.0",
		}, BuildInfo{})
		if err != nil {
			t.Fatalf("Run(regulatory label extract) failed: %v", err)
		}

		if _, err := os.Stat(filepath.Join(outDir, "bundle.yaml")); err != nil {
			t.Errorf("bundle.yaml not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(outDir, "device_identity_crop.png")); err != nil {
			t.Errorf("device_identity_crop.png not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(outDir, "giteki_mark_and_number_crop.png")); err != nil {
			t.Errorf("giteki_mark_and_number_crop.png not found: %v", err)
		}

		// Run label verify command via CLI
		err = Run([]string{"regulatory", "label", "verify", outDir}, BuildInfo{})
		if err != nil {
			t.Fatalf("Run(regulatory label verify) failed on valid bundle: %v", err)
		}

		// Tamper with bundle file and verify failure
		artFile := filepath.Join(outDir, "device_identity_crop.png")
		_ = os.WriteFile(artFile, []byte("tampered data"), 0644)
		err = Run([]string{"regulatory", "label", "verify", outDir}, BuildInfo{})
		if err == nil {
			t.Fatal("expected Run(regulatory label verify) to fail on tampered bundle, got nil")
		}
	})

	t.Run("FixedCoordinateExtractionDoesNotRunOCR", func(t *testing.T) {
		outDir := filepath.Join(tempDir, "bundle_no_ocr")
		err := Run([]string{
			"regulatory", "label", "extract",
			"--image", imgPath,
			"--layout", layoutPath,
			"--out", outDir,
			"--ocr-lang", "invalid_lang_xyz",
		}, BuildInfo{})
		if err != nil {
			t.Fatalf("fixed-coordinate extraction unexpectedly invoked OCR: %v", err)
		}
	})
}
