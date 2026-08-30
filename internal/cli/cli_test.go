package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

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
			"--layout-id", "tp-link/archer-ax23v/jp-1.0",
			"--out", outDir,
			"--crop", "device_identity_crop=200x50+10+10",
			"--crop", "giteki_mark_and_number_crop=200x80+10+70",
			"--no-ocr",
		}, BuildInfo{})
		if err == nil {
			t.Fatal("expected error for mismatched source-sha256, got nil")
		}
	})

	t.Run("MissingMandatoryCrop", func(t *testing.T) {
		outDir := filepath.Join(tempDir, "bundle_missing_crop")
		err := Run([]string{
			"regulatory", "label", "extract",
			"--image", imgPath,
			"--layout-id", "tp-link/archer-ax23v/jp-1.0",
			"--out", outDir,
			"--crop", "device_identity_crop=200x50+10+10",
			"--no-ocr",
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
			"--layout-id", "tp-link/archer-ax23v/jp-1.0",
			"--out", outDir,
			"--crop", "device_identity_crop=200x50+10+10",
			"--crop", "giteki_mark_and_number_crop=200x80+10+70",
			"--vendor", "TP-Link",
			"--model", "Archer AX23V",
			"--revision", "JP/1.0",
			"--no-ocr",
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

	t.Run("OCRFailureWithAndWithoutOptional", func(t *testing.T) {
		failDir := filepath.Join(tempDir, "bundle_ocr_fail")
		// Without --ocr-optional, invalid language should fail
		err := Run([]string{
			"regulatory", "label", "extract",
			"--image", imgPath,
			"--layout-id", "tp-link/archer-ax23v/jp-1.0",
			"--out", failDir,
			"--crop", "device_identity_crop=200x50+10+10",
			"--crop", "giteki_mark_and_number_crop=200x80+10+70",
			"--ocr-lang", "invalid_lang_xyz",
		}, BuildInfo{})
		if err == nil {
			t.Fatal("expected error without --ocr-optional, got nil")
		}

		// With --ocr-optional, it should succeed
		optDir := filepath.Join(tempDir, "bundle_ocr_opt")
		err = Run([]string{
			"regulatory", "label", "extract",
			"--image", imgPath,
			"--layout-id", "tp-link/archer-ax23v/jp-1.0",
			"--out", optDir,
			"--crop", "device_identity_crop=200x50+10+10",
			"--crop", "giteki_mark_and_number_crop=200x80+10+70",
			"--ocr-lang", "invalid_lang_xyz",
			"--ocr-optional",
		}, BuildInfo{})
		if err != nil {
			t.Fatalf("expected success with --ocr-optional, got: %v", err)
		}
	})
}
