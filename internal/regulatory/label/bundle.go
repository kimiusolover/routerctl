package label

import (
	"fmt"
	"regexp"
	"time"
)

var (
	sha256Regex     = regexp.MustCompile(`^[a-f0-9]{64}$`)
	layoutIDRegex   = regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+/[a-zA-Z0-9_.-]+$`)
	certNumberRegex = regexp.MustCompile(`^\d{3}-\d{6}$`)

	AllowedRoles = map[string]bool{
		"device_identity_crop":        true,
		"giteki_mark_and_number_crop": true,
	}

	RequiredRoles = []string{
		"device_identity_crop",
		"giteki_mark_and_number_crop",
	}
)

type SourceSpec struct {
	SHA256   string `yaml:"sha256"`
	LayoutID string `yaml:"layoutId"`
}

type ArtifactSpec struct {
	Role   string `yaml:"role"`
	File   string `yaml:"file"`
	SHA256 string `yaml:"sha256"`
}

type ReviewedSpec struct {
	Vendor              string `yaml:"vendor,omitempty"`
	Model               string `yaml:"model,omitempty"`
	HardwareRevision    string `yaml:"hardwareRevision,omitempty"`
	CertificationNumber string `yaml:"certificationNumber,omitempty"`
	Status              string `yaml:"status,omitempty"` // "confirmed", "unreviewed"
}

type OCRSpec struct {
	Status               string   `yaml:"status,omitempty"` // "success", "unavailable", "no_match", "not_run"
	Error                string   `yaml:"error,omitempty"`
	CertificationNumbers []string `yaml:"certificationNumbers,omitempty"`
}

type ObservationsSpec struct {
	OCRCandidates OCRSpec      `yaml:"ocrCandidates,omitempty"`
	Reviewed      ReviewedSpec `yaml:"reviewed,omitempty"`
}

type RegulatoryBundle struct {
	Version      string           `yaml:"version"`
	GeneratedAt  time.Time        `yaml:"generatedAt"`
	Source       SourceSpec       `yaml:"source"`
	Artifacts    []ArtifactSpec   `yaml:"artifacts"`
	Observations ObservationsSpec `yaml:"observations,omitempty"`
}

// Validate performs structural and semantic integrity checks on the bundle.
func (b *RegulatoryBundle) Validate() error {
	if b.Version != "1.0" {
		return fmt.Errorf("invalid bundle version %q, expected %q", b.Version, "1.0")
	}
	if b.GeneratedAt.IsZero() {
		return fmt.Errorf("bundle generatedAt timestamp is required")
	}

	// Validate Source
	if !sha256Regex.MatchString(b.Source.SHA256) {
		return fmt.Errorf("invalid source SHA-256 digest format: %q", b.Source.SHA256)
	}
	if !layoutIDRegex.MatchString(b.Source.LayoutID) {
		return fmt.Errorf("invalid source layoutId format: %q (expected vendor/model/revision-version)", b.Source.LayoutID)
	}

	// Validate Artifacts
	if len(b.Artifacts) == 0 {
		return fmt.Errorf("bundle must contain at least one artifact")
	}

	seenRoles := make(map[string]bool)
	for _, art := range b.Artifacts {
		if !AllowedRoles[art.Role] {
			return fmt.Errorf("unauthorized artifact role: %q", art.Role)
		}
		if seenRoles[art.Role] {
			return fmt.Errorf("duplicate artifact role: %q", art.Role)
		}
		seenRoles[art.Role] = true

		expectedFile := art.Role + ".png"
		if art.File != expectedFile {
			return fmt.Errorf("artifact file for role %q must be %q, got %q", art.Role, expectedFile, art.File)
		}
		if !sha256Regex.MatchString(art.SHA256) {
			return fmt.Errorf("invalid artifact SHA-256 digest format for role %q: %q", art.Role, art.SHA256)
		}
	}

	// Verify all mandatory roles are present
	for _, reqRole := range RequiredRoles {
		if !seenRoles[reqRole] {
			return fmt.Errorf("missing mandatory artifact role: %q", reqRole)
		}
	}

	// Validate Reviewed metadata if present
	rev := b.Observations.Reviewed
	if rev.Status != "" && rev.Status != "confirmed" && rev.Status != "unreviewed" {
		return fmt.Errorf("invalid reviewed status %q (expected 'confirmed' or 'unreviewed')", rev.Status)
	}
	if rev.CertificationNumber != "" && !certNumberRegex.MatchString(rev.CertificationNumber) {
		return fmt.Errorf("invalid reviewed certification number format %q (expected XXX-XXXXXX)", rev.CertificationNumber)
	}

	// Validate OCR state consistency
	ocr := b.Observations.OCRCandidates
	if ocr.Status != "" {
		switch ocr.Status {
		case "success":
			if len(ocr.CertificationNumbers) == 0 {
				return fmt.Errorf("OCR status 'success' requires at least one certification number candidate")
			}
			if ocr.Error != "" {
				return fmt.Errorf("OCR status 'success' cannot have an error message")
			}
		case "unavailable":
			if ocr.Error == "" {
				return fmt.Errorf("OCR status 'unavailable' requires an error message")
			}
			if len(ocr.CertificationNumbers) > 0 {
				return fmt.Errorf("OCR status 'unavailable' cannot have certification number candidates")
			}
		case "no_match":
			if len(ocr.CertificationNumbers) > 0 {
				return fmt.Errorf("OCR status 'no_match' cannot have certification number candidates")
			}
			if ocr.Error != "" {
				return fmt.Errorf("OCR status 'no_match' cannot have an error message")
			}
		case "not_run":
			if len(ocr.CertificationNumbers) > 0 {
				return fmt.Errorf("OCR status 'not_run' cannot have certification number candidates")
			}
			if ocr.Error != "" {
				return fmt.Errorf("OCR status 'not_run' cannot have an error message")
			}
		default:
			return fmt.Errorf("invalid OCR status %q (expected 'success', 'unavailable', 'no_match', or 'not_run')", ocr.Status)
		}
	} else {
		if len(ocr.CertificationNumbers) > 0 || ocr.Error != "" {
			return fmt.Errorf("OCR candidates or error cannot be specified without setting an OCR status")
		}
	}

	for _, cand := range ocr.CertificationNumbers {
		if !certNumberRegex.MatchString(cand) {
			return fmt.Errorf("invalid OCR certification candidate format %q (expected XXX-XXXXXX)", cand)
		}
	}

	return nil
}
