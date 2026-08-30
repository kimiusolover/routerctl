package regulatory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/example/routerctl/internal/regulatory/model"
)

const EvidenceBundleAPIVersion = "routerctl.regulatory.evidence-bundle/v1"

type EvidenceBundle struct {
	APIVersion          string             `json:"apiVersion"`
	Device              model.Device       `json:"device"`
	Jurisdiction        string             `json:"jurisdiction"`
	Authority           string             `json:"authority"`
	CertificationNumber string             `json:"certificationNumber"`
	NumberSource        model.NumberSource `json:"numberSource"`
	Documents           []BundleDocument   `json:"documents"`
}
type BundleDocument struct {
	Role   string `json:"role"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

func ReadBundle(file string) (*EvidenceBundle, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var bundle EvidenceBundle
	if err := json.Unmarshal(b, &bundle); err != nil {
		bundle, err = parseBundleYAML(string(b))
		if err != nil {
			return nil, fmt.Errorf("regulatory bundle: parse JSON or YAML: %w", err)
		}
	}
	if bundle.APIVersion != EvidenceBundleAPIVersion || bundle.Device.Vendor == "" || bundle.Device.Model == "" || bundle.Device.Revision == "" || bundle.Jurisdiction == "" || bundle.Authority == "" || bundle.CertificationNumber == "" || bundle.NumberSource.URL == "" || bundle.NumberSource.EvidenceFile == "" || len(bundle.Documents) == 0 {
		return nil, fmt.Errorf("regulatory bundle: missing required evidence")
	}
	if bundle.NumberSource.Type == "official_search" && (bundle.NumberSource.Retrieval != "manual" || bundle.NumberSource.CheckedAt == "" || bundle.NumberSource.CheckedBy == "" || !validMatchStatus(bundle.NumberSource.MatchStatus)) {
		return nil, fmt.Errorf("regulatory bundle: official_search requires manual retrieval, checkedAt, checkedBy, and valid matchStatus")
	}
	base := filepath.Dir(file)
	if err := verifyFile(filepath.Join(base, bundle.NumberSource.EvidenceFile), bundle.NumberSource.SHA256); err != nil {
		return nil, fmt.Errorf("regulatory bundle number source: %w", err)
	}
	for _, d := range bundle.Documents {
		if d.Role == "" || d.File == "" {
			return nil, fmt.Errorf("regulatory bundle: invalid document")
		}
		if err := verifyFile(filepath.Join(base, d.File), d.SHA256); err != nil {
			return nil, fmt.Errorf("regulatory bundle document %s: %w", d.File, err)
		}
	}
	return &bundle, nil
}

// CreateBundle hashes only files the operator selected locally; it never
// searches an authority site or fabricates a certification number.
func CreateBundle(authority, number, sourceURL, checkedAt, checkedBy, evidenceFile, report, vendor, deviceModel, revision string) (*EvidenceBundle, error) {
	evidenceHash, err := fileSHA256(evidenceFile)
	if err != nil {
		return nil, err
	}
	reportHash, err := fileSHA256(report)
	if err != nil {
		return nil, err
	}
	return &EvidenceBundle{APIVersion: EvidenceBundleAPIVersion, Device: model.Device{Vendor: vendor, Model: deviceModel, Revision: revision}, Jurisdiction: "JP", Authority: authority, CertificationNumber: number,
		NumberSource: model.NumberSource{Type: "official_search", URL: sourceURL, Retrieval: "manual", CheckedAt: checkedAt, CheckedBy: checkedBy, MatchStatus: "unconfirmed", EvidenceFile: filepath.Base(evidenceFile), SHA256: evidenceHash},
		Documents:    []BundleDocument{{Role: "test_report", File: filepath.Base(report), SHA256: reportHash}}}, nil
}
func fileSHA256(file string) (string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	got := sha256.Sum256(b)
	return hex.EncodeToString(got[:]), nil
}

// parseBundleYAML accepts the deliberately small mapping/list subset documented
// by evidence-bundle.schema.json. It is not a general YAML interpreter.
func parseBundleYAML(text string) (EvidenceBundle, error) {
	var b EvidenceBundle
	var doc *BundleDocument
	section := ""
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "device:" {
			section = "device"
			continue
		}
		if line == "numberSource:" {
			section = "number"
			continue
		}
		if line == "documents:" {
			section = "documents"
			continue
		}
		if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") && !strings.HasPrefix(line, "- ") {
			section = ""
		}
		if strings.HasPrefix(line, "- ") {
			b.Documents = append(b.Documents, BundleDocument{})
			doc = &b.Documents[len(b.Documents)-1]
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
			section = "document"
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return b, fmt.Errorf("invalid YAML line %q", line)
		}
		key, value := strings.TrimSpace(parts[0]), strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		if section == "number" {
			switch key {
			case "type":
				b.NumberSource.Type = value
			case "url":
				b.NumberSource.URL = value
			case "retrieval":
				b.NumberSource.Retrieval = value
			case "checkedAt":
				b.NumberSource.CheckedAt = value
			case "checkedBy":
				b.NumberSource.CheckedBy = value
			case "matchStatus":
				b.NumberSource.MatchStatus = value
			case "evidenceFile":
				b.NumberSource.EvidenceFile = value
			case "sha256":
				b.NumberSource.SHA256 = value
			}
			continue
		}
		if section == "device" {
			switch key {
			case "vendor":
				b.Device.Vendor = value
			case "model":
				b.Device.Model = value
			case "revision":
				b.Device.Revision = value
			}
			continue
		}
		if section == "document" && doc != nil {
			switch key {
			case "role":
				doc.Role = value
			case "file":
				doc.File = value
			case "sha256":
				doc.SHA256 = value
			}
			continue
		}
		switch key {
		case "apiVersion":
			b.APIVersion = value
		case "jurisdiction":
			b.Jurisdiction = value
		case "authority":
			b.Authority = value
		case "certificationNumber":
			b.CertificationNumber = value
		}
	}
	return b, nil
}
func validMatchStatus(value string) bool {
	return value == "unconfirmed" || value == "confirmed" || value == "mismatch"
}
func verifyFile(file, want string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	got := sha256.Sum256(b)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("SHA-256 mismatch")
	}
	return nil
}
