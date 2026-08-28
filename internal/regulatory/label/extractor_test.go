package label_test

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/routerctl/internal/regulatory/label"
)

func createTestPNGImage(t *testing.T, dir string, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: 255})
		}
	}
	imgPath := filepath.Join(dir, "test_label.png")
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatalf("failed to create test image file: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("failed to encode test png: %v", err)
	}
	return imgPath
}

func createTestJPEGImage(t *testing.T, dir string, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: 255})
		}
	}
	imgPath := filepath.Join(dir, "test_label.jpg")
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatalf("failed to create test jpeg file: %v", err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatalf("failed to encode test jpeg: %v", err)
	}
	return imgPath
}

func computeFileSHA256(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file for hash: %v", err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestValidateImageSource(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("ValidPNG", func(t *testing.T) {
		pngPath := createTestPNGImage(t, tempDir, 100, 100)
		if err := label.ValidateImageSource(pngPath); err != nil {
			t.Errorf("expected valid PNG to pass validation, got: %v", err)
		}
	})

	t.Run("ValidJPEG", func(t *testing.T) {
		jpgPath := createTestJPEGImage(t, tempDir, 100, 100)
		if err := label.ValidateImageSource(jpgPath); err != nil {
			t.Errorf("expected valid JPEG to pass validation, got: %v", err)
		}
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		err := label.ValidateImageSource(filepath.Join(tempDir, "does_not_exist.png"))
		if err == nil {
			t.Error("expected error for non-existent file, got nil")
		}
	})

	t.Run("DirectoryAsSource", func(t *testing.T) {
		err := label.ValidateImageSource(tempDir)
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Errorf("expected directory rejection error, got: %v", err)
		}
	})

	t.Run("SymlinkSourceImageRejection", func(t *testing.T) {
		realPNG := createTestPNGImage(t, tempDir, 100, 100)
		symlinkPath := filepath.Join(tempDir, "symlink_source.png")
		if err := os.Symlink(realPNG, symlinkPath); err != nil {
			t.Fatal(err)
		}

		err := label.ValidateImageSource(symlinkPath)
		if err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Errorf("expected symlink source rejection error, got: %v", err)
		}
	})

	t.Run("DangerousProtocolPrefixes", func(t *testing.T) {
		prefixes := []string{
			"http://example.com/test.png",
			"https://example.com/test.png",
			"null:white",
			"caption:hello",
			"label:test",
		}
		for _, p := range prefixes {
			if err := label.ValidateImageSource(p); err == nil {
				t.Errorf("expected prefix %q to be rejected, got nil", p)
			}
		}
	})

	t.Run("InvalidMagicBytes_PlainText", func(t *testing.T) {
		fakePNG := filepath.Join(tempDir, "fake.png")
		if err := os.WriteFile(fakePNG, []byte("Hello, I am actually a text file disguised as PNG!"), 0644); err != nil {
			t.Fatal(err)
		}
		err := label.ValidateImageSource(fakePNG)
		if err == nil || !strings.Contains(err.Error(), "magic bytes mismatch") {
			t.Errorf("expected magic bytes mismatch error, got: %v", err)
		}
	})

	t.Run("InvalidMagicBytes_PDF", func(t *testing.T) {
		fakePDF := filepath.Join(tempDir, "document.pdf")
		if err := os.WriteFile(fakePDF, []byte("%PDF-1.4\n...fake pdf..."), 0644); err != nil {
			t.Fatal(err)
		}
		err := label.ValidateImageSource(fakePDF)
		if err == nil || !strings.Contains(err.Error(), "magic bytes mismatch") {
			t.Errorf("expected magic bytes mismatch error, got: %v", err)
		}
	})
}

func TestExtractAndCommitBundle_ValidationErrors(t *testing.T) {
	tempDir := t.TempDir()
	imgPath := createTestPNGImage(t, tempDir, 500, 300)
	ctx := context.Background()

	t.Run("DestinationAlreadyExists", func(t *testing.T) {
		existingDir := filepath.Join(tempDir, "existing_bundle")
		if err := os.Mkdir(existingDir, 0755); err != nil {
			t.Fatal(err)
		}

		_, err := label.ExtractAndCommitBundle(ctx, imgPath, "", "tp-link/ax23v/1.0", existingDir, []label.CropTarget{
			{Role: "device_identity_crop", Geometry: "100x50+10+10"},
			{Role: "giteki_mark_and_number_crop", Geometry: "100x50+10+70"},
		})
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected error for existing destination directory, got: %v", err)
		}
	})

	t.Run("SourceSHA256Mismatch", func(t *testing.T) {
		destDir := filepath.Join(tempDir, "sha_mismatch_bundle")
		wrongSHA := "0000000000000000000000000000000000000000000000000000000000000000"
		_, err := label.ExtractAndCommitBundle(ctx, imgPath, wrongSHA, "tp-link/ax23v/1.0", destDir, []label.CropTarget{
			{Role: "device_identity_crop", Geometry: "100x50+10+10"},
			{Role: "giteki_mark_and_number_crop", Geometry: "100x50+10+70"},
		})
		if err == nil || !strings.Contains(err.Error(), "source SHA-256 digest mismatch") {
			t.Errorf("expected source SHA-256 digest mismatch error, got: %v", err)
		}
	})

	t.Run("MissingMandatoryRole", func(t *testing.T) {
		destDir := filepath.Join(tempDir, "missing_role_bundle")
		// Provide only device_identity_crop, omitting giteki_mark_and_number_crop
		_, err := label.ExtractAndCommitBundle(ctx, imgPath, "", "tp-link/ax23v/1.0", destDir, []label.CropTarget{
			{Role: "device_identity_crop", Geometry: "100x50+10+10"},
		})
		if err == nil || !strings.Contains(err.Error(), "missing mandatory crop role") {
			t.Errorf("expected missing mandatory crop role error, got: %v", err)
		}
	})

	t.Run("UnauthorizedRole", func(t *testing.T) {
		destDir := filepath.Join(tempDir, "unauth_bundle")
		_, err := label.ExtractAndCommitBundle(ctx, imgPath, "", "tp-link/ax23v/1.0", destDir, []label.CropTarget{
			{Role: "secret_serial_number_crop", Geometry: "100x50+10+10"},
			{Role: "device_identity_crop", Geometry: "100x50+10+70"},
		})
		if err == nil || !strings.Contains(err.Error(), "unauthorized") {
			t.Errorf("expected error for unauthorized role, got: %v", err)
		}
	})

	t.Run("DuplicateRole", func(t *testing.T) {
		destDir := filepath.Join(tempDir, "dup_bundle")
		_, err := label.ExtractAndCommitBundle(ctx, imgPath, "", "tp-link/ax23v/1.0", destDir, []label.CropTarget{
			{Role: "device_identity_crop", Geometry: "100x50+10+10"},
			{Role: "device_identity_crop", Geometry: "100x50+20+20"},
		})
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("expected error for duplicate role, got: %v", err)
		}
	})

	t.Run("InvalidGeometryFormat", func(t *testing.T) {
		destDir := filepath.Join(tempDir, "invalid_geo_bundle")
		_, err := label.ExtractAndCommitBundle(ctx, imgPath, "", "tp-link/ax23v/1.0", destDir, []label.CropTarget{
			{Role: "device_identity_crop", Geometry: "100x50"},
			{Role: "giteki_mark_and_number_crop", Geometry: "100x50+10+70"},
		})
		if err == nil || !strings.Contains(err.Error(), "invalid geometry format") {
			t.Errorf("expected error for invalid geometry format, got: %v", err)
		}
	})

	t.Run("BoundaryExceeded", func(t *testing.T) {
		destDir := filepath.Join(tempDir, "overflow_bundle")
		// Image is 500x300, crop width 450 + x 100 = 550 > 500
		_, err := label.ExtractAndCommitBundle(ctx, imgPath, "", "tp-link/ax23v/1.0", destDir, []label.CropTarget{
			{Role: "device_identity_crop", Geometry: "450x100+100+10"},
			{Role: "giteki_mark_and_number_crop", Geometry: "100x50+10+10"},
		})
		if err == nil || !strings.Contains(err.Error(), "exceeds image boundary") {
			t.Errorf("expected error for geometry exceeding image bounds, got: %v", err)
		}
	})
}

func TestExtractAndCommitBundle_Success(t *testing.T) {
	tempDir := t.TempDir()
	imgPath := createTestPNGImage(t, tempDir, 800, 600)
	expectedSHA256 := computeFileSHA256(t, imgPath)
	ctx := context.Background()

	finalDir := filepath.Join(tempDir, "output", "test_bundle")
	crops := []label.CropTarget{
		{Role: "giteki_mark_and_number_crop", Geometry: "300x150+50+200"},
		{Role: "device_identity_crop", Geometry: "400x100+50+50"},
	}

	bundle, err := label.ExtractAndCommitBundleWithOptions(ctx, label.ExtractOptions{
		SourcePath:   imgPath,
		SourceSHA256: expectedSHA256, // verify explicit SHA-256 match
		LayoutID:     "tp-link/archer-ax23v/jp-1.0",
		FinalDir:     finalDir,
		Crops:        crops,
		Reviewed: label.ReviewedSpec{
			Vendor: "TP-Link",
			Model:  "Archer AX23V",
			Status: "unreviewed",
		},
	})
	if err != nil {
		t.Fatalf("ExtractAndCommitBundle failed: %v", err)
	}

	if bundle.Version != "1.0" {
		t.Errorf("expected version 1.0, got %q", bundle.Version)
	}
	if bundle.Source.LayoutID != "tp-link/archer-ax23v/jp-1.0" {
		t.Errorf("expected layoutId 'tp-link/archer-ax23v/jp-1.0', got %q", bundle.Source.LayoutID)
	}
	if bundle.Source.SHA256 != expectedSHA256 {
		t.Errorf("expected source SHA-256 %q, got %q", expectedSHA256, bundle.Source.SHA256)
	}

	// Verify deterministic sorting: device_identity_crop before giteki_mark_and_number_crop
	if len(bundle.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(bundle.Artifacts))
	}
	if bundle.Artifacts[0].Role != "device_identity_crop" {
		t.Errorf("expected first artifact role 'device_identity_crop', got %q", bundle.Artifacts[0].Role)
	}
	if bundle.Artifacts[1].Role != "giteki_mark_and_number_crop" {
		t.Errorf("expected second artifact role 'giteki_mark_and_number_crop', got %q", bundle.Artifacts[1].Role)
	}

	// Verify the bundle directory using VerifyBundleDirectory
	verifiedBundle, err := label.VerifyBundleDirectory(finalDir)
	if err != nil {
		t.Fatalf("VerifyBundleDirectory failed on valid bundle: %v", err)
	}
	if verifiedBundle.Source.LayoutID != "tp-link/archer-ax23v/jp-1.0" {
		t.Errorf("verified bundle layout mismatch: %s", verifiedBundle.Source.LayoutID)
	}
}

func TestExtractAndCommitBundle_WithEmbeddedFixture_AndOCR(t *testing.T) {
	tempDir := t.TempDir()
	imgPath := filepath.Join(tempDir, "synthetic_fixture.png")
	if err := os.WriteFile(imgPath, syntheticLabelPNG, 0644); err != nil {
		t.Fatalf("failed to write synthetic fixture: %v", err)
	}

	setupTessdata(t, tempDir)

	finalDir := filepath.Join(tempDir, "ocr_bundle")
	crops := []label.CropTarget{
		{Role: "device_identity_crop", Geometry: "400x100+20+20"},
		{Role: "giteki_mark_and_number_crop", Geometry: "400x150+20+150"},
	}

	bundle, err := label.ExtractAndCommitBundleWithOptions(context.Background(), label.ExtractOptions{
		SourcePath:  imgPath,
		LayoutID:    "tp-link/archer-ax23v/jp-1.0",
		FinalDir:    finalDir,
		Crops:       crops,
		RunOCR:      true,
		OCROptional: false,
		Reviewed: label.ReviewedSpec{
			Vendor:              "TP-Link",
			Model:               "Archer AX23V",
			HardwareRevision:    "JP/1.0",
			CertificationNumber: "201-230283",
			Status:              "confirmed",
		},
	})
	if err != nil {
		t.Fatalf("extraction with OCR failed: %v", err)
	}

	if bundle.Observations.OCRCandidates.Status != "success" {
		t.Errorf("expected OCR status 'success', got %q", bundle.Observations.OCRCandidates.Status)
	}
	if len(bundle.Observations.OCRCandidates.CertificationNumbers) != 1 || bundle.Observations.OCRCandidates.CertificationNumbers[0] != "201-230283" {
		t.Errorf("expected OCR candidate ['201-230283'], got: %v", bundle.Observations.OCRCandidates.CertificationNumbers)
	}

	if err := bundle.Validate(); err != nil {
		t.Fatalf("bundle.Validate failed: %v", err)
	}

	// Verify directory integrity
	if _, err := label.VerifyBundleDirectory(finalDir); err != nil {
		t.Fatalf("VerifyBundleDirectory failed: %v", err)
	}
}

func TestExtractAndCommitBundle_OCRFailureWithoutOptionalFlag(t *testing.T) {
	tempDir := t.TempDir()
	imgPath := createTestPNGImage(t, tempDir, 500, 300)
	finalDir := filepath.Join(tempDir, "fail_bundle")
	crops := []label.CropTarget{
		{Role: "device_identity_crop", Geometry: "200x50+10+10"},
		{Role: "giteki_mark_and_number_crop", Geometry: "200x50+10+70"},
	}

	// Deliberately request a non-existent language without OCROptional
	_, err := label.ExtractAndCommitBundleWithOptions(context.Background(), label.ExtractOptions{
		SourcePath:  imgPath,
		LayoutID:    "tp-link/archer-ax23v/jp-1.0",
		FinalDir:    finalDir,
		Crops:       crops,
		RunOCR:      true,
		OCROptional: false,
		OCRLang:     "non_existent_lang_xyz",
	})
	if err == nil || !strings.Contains(err.Error(), "OCR candidate extraction failed") {
		t.Errorf("expected OCR failure error when OCROptional is false, got: %v", err)
	}

	// With OCROptional true, it should record status: "unavailable" and error
	bundle, err := label.ExtractAndCommitBundleWithOptions(context.Background(), label.ExtractOptions{
		SourcePath:  imgPath,
		LayoutID:    "tp-link/archer-ax23v/jp-1.0",
		FinalDir:    finalDir,
		Crops:       crops,
		RunOCR:      true,
		OCROptional: true,
		OCRLang:     "non_existent_lang_xyz",
	})
	if err != nil {
		t.Errorf("expected success with OCROptional true, got error: %v", err)
	}
	if bundle.Observations.OCRCandidates.Status != "unavailable" {
		t.Errorf("expected OCR status 'unavailable', got %q", bundle.Observations.OCRCandidates.Status)
	}
	if bundle.Observations.OCRCandidates.Error == "" {
		t.Errorf("expected OCR error message to be recorded in bundle")
	}
}

func TestVerifyBundleDirectory_IntegrityChecks(t *testing.T) {
	tempDir := t.TempDir()
	imgPath := createTestPNGImage(t, tempDir, 500, 300)
	bundleDir := filepath.Join(tempDir, "valid_bundle")
	crops := []label.CropTarget{
		{Role: "device_identity_crop", Geometry: "200x50+10+10"},
		{Role: "giteki_mark_and_number_crop", Geometry: "200x50+10+70"},
	}

	_, err := label.ExtractAndCommitBundleWithOptions(context.Background(), label.ExtractOptions{
		SourcePath: imgPath,
		LayoutID:   "tp-link/archer-ax23v/jp-1.0",
		FinalDir:   bundleDir,
		Crops:      crops,
		RunOCR:     false,
	})
	if err != nil {
		t.Fatalf("bundle creation failed: %v", err)
	}

	t.Run("ValidBundlePasses", func(t *testing.T) {
		b, err := label.VerifyBundleDirectory(bundleDir)
		if err != nil {
			t.Fatalf("expected valid bundle to pass verification: %v", err)
		}
		if b == nil {
			t.Fatal("expected non-nil bundle")
		}
	})

	t.Run("SymlinkedBundleDirectoryRejection", func(t *testing.T) {
		symDir := filepath.Join(tempDir, "symlink_bundle_dir")
		if err := os.Symlink(bundleDir, symDir); err != nil {
			t.Fatal(err)
		}

		_, err := label.VerifyBundleDirectory(symDir)
		if err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Errorf("expected symlinked bundle directory error, got: %v", err)
		}
	})

	t.Run("SymlinkedBundleYAMLRejection", func(t *testing.T) {
		symYAMLDir := filepath.Join(tempDir, "sym_yaml_bundle")
		if err := copyDir(bundleDir, symYAMLDir); err != nil {
			t.Fatal(err)
		}

		// Replace bundle.yaml with symlink to /outside/bundle.yaml
		outsideYAML := filepath.Join(tempDir, "outside_bundle.yaml")
		data, _ := os.ReadFile(filepath.Join(symYAMLDir, "bundle.yaml"))
		_ = os.WriteFile(outsideYAML, data, 0644)
		_ = os.Remove(filepath.Join(symYAMLDir, "bundle.yaml"))
		if err := os.Symlink(outsideYAML, filepath.Join(symYAMLDir, "bundle.yaml")); err != nil {
			t.Fatal(err)
		}

		_, err := label.VerifyBundleDirectory(symYAMLDir)
		if err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Errorf("expected symlinked bundle.yaml error, got: %v", err)
		}
	})

	t.Run("SymlinkedArtifactRejection", func(t *testing.T) {
		symArtDir := filepath.Join(tempDir, "sym_art_bundle")
		if err := copyDir(bundleDir, symArtDir); err != nil {
			t.Fatal(err)
		}

		// Move device_identity_crop.png outside and create symlink
		outsidePNG := filepath.Join(tempDir, "outside_crop.png")
		artPath := filepath.Join(symArtDir, "device_identity_crop.png")
		pngData, _ := os.ReadFile(artPath)
		_ = os.WriteFile(outsidePNG, pngData, 0644)
		_ = os.Remove(artPath)
		if err := os.Symlink(outsidePNG, artPath); err != nil {
			t.Fatal(err)
		}

		_, err := label.VerifyBundleDirectory(symArtDir)
		if err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Errorf("expected symlinked artifact rejection error, got: %v", err)
		}
	})

	t.Run("TamperedArtifactDigestMismatch", func(t *testing.T) {
		tamperDir := filepath.Join(tempDir, "tampered_bundle")
		if err := copyDir(bundleDir, tamperDir); err != nil {
			t.Fatal(err)
		}

		// Alter device_identity_crop.png
		artPath := filepath.Join(tamperDir, "device_identity_crop.png")
		if err := os.WriteFile(artPath, []byte("tampered data bytes"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := label.VerifyBundleDirectory(tamperDir)
		if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
			t.Errorf("expected digest mismatch error, got: %v", err)
		}
	})

	t.Run("MissingArtifactFile", func(t *testing.T) {
		missingDir := filepath.Join(tempDir, "missing_art_bundle")
		if err := copyDir(bundleDir, missingDir); err != nil {
			t.Fatal(err)
		}

		if err := os.Remove(filepath.Join(missingDir, "device_identity_crop.png")); err != nil {
			t.Fatal(err)
		}

		_, err := label.VerifyBundleDirectory(missingDir)
		if err == nil || !strings.Contains(err.Error(), "missing from bundle") {
			t.Errorf("expected missing artifact file error, got: %v", err)
		}
	})

	t.Run("ExtraneousUnauthorizedFile", func(t *testing.T) {
		extraDir := filepath.Join(tempDir, "extra_file_bundle")
		if err := copyDir(bundleDir, extraDir); err != nil {
			t.Fatal(err)
		}

		// Add an extraneous file
		if err := os.WriteFile(filepath.Join(extraDir, "secret_key.txt"), []byte("secret"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := label.VerifyBundleDirectory(extraDir)
		if err == nil || !strings.Contains(err.Error(), "extraneous unauthorized file") {
			t.Errorf("expected extraneous file error, got: %v", err)
		}
	})

	t.Run("ExtraneousSymlinkInDirectory", func(t *testing.T) {
		symEntryDir := filepath.Join(tempDir, "extra_symlink_bundle")
		if err := copyDir(bundleDir, symEntryDir); err != nil {
			t.Fatal(err)
		}

		// Add an unauthorized symlink inside bundle directory
		targetFile := filepath.Join(tempDir, "target.txt")
		_ = os.WriteFile(targetFile, []byte("target"), 0644)
		if err := os.Symlink(targetFile, filepath.Join(symEntryDir, "extra_symlink")); err != nil {
			t.Fatal(err)
		}

		_, err := label.VerifyBundleDirectory(symEntryDir)
		if err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Errorf("expected symlink entry rejection error, got: %v", err)
		}
	})

	t.Run("PathTraversalInManifest", func(t *testing.T) {
		traversalDir := filepath.Join(tempDir, "traversal_bundle")
		if err := copyDir(bundleDir, traversalDir); err != nil {
			t.Fatal(err)
		}

		// Edit bundle.yaml to include a path traversal in artifact.file
		manifestPath := filepath.Join(traversalDir, "bundle.yaml")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		modifiedData := strings.Replace(string(data), "device_identity_crop.png", "../device_identity_crop.png", 1)
		if err := os.WriteFile(manifestPath, []byte(modifiedData), 0644); err != nil {
			t.Fatal(err)
		}

		_, err = label.VerifyBundleDirectory(traversalDir)
		if err == nil {
			t.Errorf("expected path traversal rejection error, got nil")
		}
	})
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}
