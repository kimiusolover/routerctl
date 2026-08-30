package label_test

import (
	"strings"
	"testing"
	"time"

	"github.com/example/routerctl/internal/regulatory/label"
	"gopkg.in/yaml.v3"
)

func validBundleFixture() label.RegulatoryBundle {
	now := time.Date(2026, 8, 29, 1, 0, 0, 0, time.UTC)
	b := label.RegulatoryBundle{
		Version:     "1.0",
		GeneratedAt: now,
		Source: label.SourceSpec{
			SHA256:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			LayoutID: "tp-link/archer-ax23v/jp-1.0",
		},
		Artifacts: []label.ArtifactSpec{
			{
				Role:   "device_identity_crop",
				File:   "device_identity_crop.png",
				SHA256: "8f43c0ec98fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
			{
				Role:   "giteki_mark_and_number_crop",
				File:   "giteki_mark_and_number_crop.png",
				SHA256: "4a2b91f098fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
		},
	}
	b.Observations.OCRCandidates = label.OCRSpec{
		Status:               "success",
		CertificationNumbers: []string{"201-230283"},
	}
	b.Observations.Reviewed = label.ReviewedSpec{
		Vendor:              "TP-Link",
		Model:               "Archer AX23V",
		HardwareRevision:    "JP/1.0",
		CertificationNumber: "201-230283",
		Status:              "confirmed",
	}
	return b
}

func TestRegulatoryBundle_YAMLSerialization(t *testing.T) {
	bundle := validBundleFixture()

	data, err := yaml.Marshal(&bundle)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	var parsed label.RegulatoryBundle
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal YAML: %v", err)
	}

	if err := parsed.Validate(); err != nil {
		t.Fatalf("parsed bundle failed validation: %v", err)
	}

	if parsed.Version != "1.0" {
		t.Errorf("expected version '1.0', got %q", parsed.Version)
	}
	if parsed.Source.LayoutID != "tp-link/archer-ax23v/jp-1.0" {
		t.Errorf("expected layoutId 'tp-link/archer-ax23v/jp-1.0', got %q", parsed.Source.LayoutID)
	}
	if len(parsed.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(parsed.Artifacts))
	}
	if parsed.Observations.OCRCandidates.Status != "success" {
		t.Errorf("expected OCR status 'success', got %q", parsed.Observations.OCRCandidates.Status)
	}
	if len(parsed.Observations.OCRCandidates.CertificationNumbers) != 1 || parsed.Observations.OCRCandidates.CertificationNumbers[0] != "201-230283" {
		t.Errorf("unexpected OCR candidates: %v", parsed.Observations.OCRCandidates.CertificationNumbers)
	}
	if parsed.Observations.Reviewed.Model != "Archer AX23V" {
		t.Errorf("expected reviewed model 'Archer AX23V', got %q", parsed.Observations.Reviewed.Model)
	}
}

func TestRegulatoryBundle_Validation(t *testing.T) {
	t.Run("ValidBundle", func(t *testing.T) {
		b := validBundleFixture()
		if err := b.Validate(); err != nil {
			t.Errorf("expected valid bundle, got error: %v", err)
		}
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		b := validBundleFixture()
		b.Version = "2.0"
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "version") {
			t.Errorf("expected version error, got %v", err)
		}
	})

	t.Run("MissingGeneratedAt", func(t *testing.T) {
		b := validBundleFixture()
		b.GeneratedAt = time.Time{}
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "generatedAt") {
			t.Errorf("expected generatedAt error, got %v", err)
		}
	})

	t.Run("InvalidSourceSHA256", func(t *testing.T) {
		b := validBundleFixture()
		b.Source.SHA256 = "invalid-sha"
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "SHA-256") {
			t.Errorf("expected SHA-256 format error, got %v", err)
		}
	})

	t.Run("InvalidLayoutID", func(t *testing.T) {
		b := validBundleFixture()
		b.Source.LayoutID = "invalid layout id with spaces"
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "layoutId") {
			t.Errorf("expected layoutId format error, got %v", err)
		}
	})

	t.Run("MissingMandatoryRole", func(t *testing.T) {
		b := validBundleFixture()
		// Remove giteki_mark_and_number_crop
		b.Artifacts = []label.ArtifactSpec{b.Artifacts[0]}
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "missing mandatory artifact role") {
			t.Errorf("expected missing mandatory role error, got %v", err)
		}
	})

	t.Run("UnauthorizedArtifactRole", func(t *testing.T) {
		b := validBundleFixture()
		b.Artifacts = append(b.Artifacts, label.ArtifactSpec{
			Role:   "wifi_password_crop",
			File:   "wifi_password_crop.png",
			SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		})
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "unauthorized") {
			t.Errorf("expected unauthorized role error, got %v", err)
		}
	})

	t.Run("InvalidArtifactFileName", func(t *testing.T) {
		b := validBundleFixture()
		b.Artifacts[0].File = "wrong_name.png"
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "must be") {
			t.Errorf("expected file name mismatch error, got %v", err)
		}
	})

	t.Run("InvalidReviewedStatus", func(t *testing.T) {
		b := validBundleFixture()
		b.Observations.Reviewed.Status = "pending_approval"
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "reviewed status") {
			t.Errorf("expected reviewed status error, got %v", err)
		}
	})

	t.Run("InvalidReviewedCertNumber", func(t *testing.T) {
		b := validBundleFixture()
		b.Observations.Reviewed.CertificationNumber = "INVALID-123"
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "certification number") {
			t.Errorf("expected certification number format error, got %v", err)
		}
	})

	t.Run("OCRStateConsistency_SuccessRequiresCandidates", func(t *testing.T) {
		b := validBundleFixture()
		b.Observations.OCRCandidates = label.OCRSpec{
			Status:               "success",
			CertificationNumbers: nil, // empty candidates with status "success"
		}
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "requires at least one certification number") {
			t.Errorf("expected candidate requirement error, got %v", err)
		}
	})

	t.Run("OCRStateConsistency_SuccessForbidsError", func(t *testing.T) {
		b := validBundleFixture()
		b.Observations.OCRCandidates = label.OCRSpec{
			Status:               "success",
			CertificationNumbers: []string{"201-230283"},
			Error:                "some error occurred",
		}
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "cannot have an error message") {
			t.Errorf("expected error conflict error, got %v", err)
		}
	})

	t.Run("OCRStateConsistency_UnavailableRequiresError", func(t *testing.T) {
		b := validBundleFixture()
		b.Observations.OCRCandidates = label.OCRSpec{
			Status: "unavailable",
			Error:  "", // empty error
		}
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "requires an error message") {
			t.Errorf("expected error requirement, got %v", err)
		}
	})

	t.Run("OCRStateConsistency_UnavailableForbidsCandidates", func(t *testing.T) {
		b := validBundleFixture()
		b.Observations.OCRCandidates = label.OCRSpec{
			Status:               "unavailable",
			Error:                "tesseract failed",
			CertificationNumbers: []string{"201-230283"},
		}
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "cannot have certification number candidates") {
			t.Errorf("expected candidate conflict error, got %v", err)
		}
	})

	t.Run("OCRStateConsistency_NoMatchForbidsCandidates", func(t *testing.T) {
		b := validBundleFixture()
		b.Observations.OCRCandidates = label.OCRSpec{
			Status:               "no_match",
			CertificationNumbers: []string{"201-230283"},
		}
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "cannot have certification number candidates") {
			t.Errorf("expected candidate conflict error on no_match, got %v", err)
		}
	})

	t.Run("OCRStateConsistency_NotRunForbidsCandidates", func(t *testing.T) {
		b := validBundleFixture()
		b.Observations.OCRCandidates = label.OCRSpec{
			Status:               "not_run",
			CertificationNumbers: []string{"201-230283"},
		}
		if err := b.Validate(); err == nil || !strings.Contains(err.Error(), "cannot have certification number candidates") {
			t.Errorf("expected candidate conflict error on not_run, got %v", err)
		}
	})
}
