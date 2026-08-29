// Package profile defines a fail-closed certification profile.  It keeps
// statutory limits, certified limits, and test observations separate: only
// the first two can participate in an RF permission decision.
package profile

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const APIVersion = "routerctl.regulatory.certification-profile/v1"

type Profile struct {
	APIVersion  string      `yaml:"apiVersion" json:"apiVersion"`
	Kind        string      `yaml:"kind" json:"kind"`
	Metadata    Metadata    `yaml:"metadata" json:"metadata"`
	Subject     Subject     `yaml:"subject" json:"subject"`
	Constraints Constraints `yaml:"constraints" json:"constraints"`
	Evidence    Evidence    `yaml:"evidence" json:"evidence"`
	Provenance  Provenance  `yaml:"provenance" json:"provenance"`
}

type Metadata struct {
	ID             string `yaml:"id" json:"id"`
	EvidenceStatus string `yaml:"evidenceStatus" json:"evidenceStatus"`
}
type Subject struct {
	Jurisdiction  string        `yaml:"jurisdiction" json:"jurisdiction"`
	Authority     string        `yaml:"authority" json:"authority"`
	Certification Certification `yaml:"certification" json:"certification"`
	Hardware      Hardware      `yaml:"hardware" json:"hardware"`
}
type Certification struct {
	Scheme string `yaml:"scheme" json:"scheme"`
	Number string `yaml:"number" json:"number"`
}
type Hardware struct {
	Manufacturer        string `yaml:"manufacturer" json:"manufacturer"`
	Model               string `yaml:"model" json:"model"`
	Revision            string `yaml:"revision" json:"revision"`
	CalibrationRequired bool   `yaml:"calibrationRequired" json:"calibrationRequired"`
}
type Constraints struct {
	Spectrum    map[string]Band `yaml:"spectrum" json:"spectrum"`
	DFS         DFS             `yaml:"dfs" json:"dfs"`
	Enforcement Enforcement     `yaml:"enforcement" json:"enforcement"`
}
type Band struct {
	LegalRangeMHz   []float64 `yaml:"legalRangeMHz" json:"legalRangeMHz"`
	PrimaryChannels []int     `yaml:"primaryChannels" json:"primaryChannels"`
	WidthsMHz       []int     `yaml:"widthsMHz" json:"widthsMHz"`
	IndoorOnly      bool      `yaml:"indoorOnly" json:"indoorOnly"`
	DFSRequired     bool      `yaml:"dfsRequired" json:"dfsRequired"`
	Power           Power     `yaml:"power" json:"power"`
}

// Regulatory and certifiedRated are policy inputs. Measured is evidence only
// and deliberately has no role in calculating a runtime maximum.
type Power struct {
	Regulatory     map[int]float64    `yaml:"regulatoryMWPerMHz" json:"regulatoryMWPerMHz"`
	CertifiedRated map[int]float64    `yaml:"certifiedRatedMWPerMHz" json:"certifiedRatedMWPerMHz"`
	Measured       map[string]float64 `yaml:"measuredMWPerMHz,omitempty" json:"measuredMWPerMHz,omitempty"`
}
type DFS struct {
	CACMinSeconds              int     `yaml:"cacMinSeconds" json:"cacMinSeconds"`
	ChannelMoveMaxSeconds      int     `yaml:"channelMoveMaxSeconds" json:"channelMoveMaxSeconds"`
	ChannelClosingTXMaxSeconds float64 `yaml:"channelClosingTXMaxSeconds" json:"channelClosingTXMaxSeconds"`
	NonOccupancyMinSeconds     int     `yaml:"nonOccupancyMinSeconds" json:"nonOccupancyMinSeconds"`
	BypassAllowed              bool    `yaml:"bypassAllowed" json:"bypassAllowed"`
}
type Enforcement struct {
	Default                string `yaml:"default" json:"default"`
	OnUnknownCertification string `yaml:"onUnknownCertification" json:"onUnknownCertification"`
	OnHardwareMismatch     string `yaml:"onHardwareMismatch" json:"onHardwareMismatch"`
	OnCalibrationFailure   string `yaml:"onCalibrationFailure" json:"onCalibrationFailure"`
}
type Evidence struct {
	CertificationReports []string `yaml:"certificationReports" json:"certificationReports"`
	TestedSoftware       []string `yaml:"testedSoftware,omitempty" json:"testedSoftware,omitempty"`
}
type Provenance struct {
	LegalSources         []Source `yaml:"legalSources" json:"legalSources"`
	CertificationSources []Source `yaml:"certificationSources" json:"certificationSources"`
}
type Source struct {
	ID        string `yaml:"id" json:"id"`
	Reference string `yaml:"reference" json:"reference"`
}

func Read(path string) (*Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("certification profile: parse YAML: %w", err)
	}
	return &p, nil
}

// TXPermitted is intentionally conservative. A profile without reviewed
// evidence can be inspected and tested, but cannot authorize RF transmission.
func (p *Profile) TXPermitted() bool {
	return p != nil && p.Metadata.EvidenceStatus == "verified" && p.Constraints.Enforcement.Default == "deny"
}
