// Package model defines evidence-preserving regulatory records.
package model

const APIVersion = "routerctl.regulatory/v1"

const (
	MeaningMeasured        = "measured"
	MeaningConfigured      = "configured"
	MeaningCertifiedMax    = "certified_max"
	MeaningRegulatoryLimit = "regulatory_limit"
	MeaningEIRP            = "eirp"
	MeaningConductedPower  = "conducted_power"
)

// CertificationRecord holds observations from one certification document. It
// intentionally does not claim that an observed test value is a firmware
// setting; consumers must inspect Value.Meaning before deriving a limit.
type CertificationRecord struct {
	APIVersion    string        `json:"apiVersion"`
	Kind          string        `json:"kind"`
	Device        Device        `json:"device"`
	Jurisdiction  string        `json:"jurisdiction"`
	Certification Certification `json:"certification"`
	Radios        []Radio       `json:"radios"`
}

type Device struct {
	Vendor   string `json:"vendor"`
	Model    string `json:"model"`
	Revision string `json:"revision"`
}

type Certification struct {
	Authority    string       `json:"authority"`
	Number       string       `json:"number"`
	NumberSource NumberSource `json:"numberSource"`
	Source       Source       `json:"source"`
}

// NumberSource is the local, reviewable evidence for the human-supplied
// authority reference number. A URL alone is not evidence because its content
// can change; EvidenceFile and SHA256 bind the number to a saved snapshot.
type NumberSource struct {
	Type         string `json:"type"`
	URL          string `json:"url"`
	Retrieval    string `json:"retrieval"`
	CheckedAt    string `json:"checkedAt"`
	CheckedBy    string `json:"checkedBy"`
	MatchStatus  string `json:"matchStatus"`
	EvidenceFile string `json:"evidenceFile"`
	SHA256       string `json:"sha256"`
}

type Source struct {
	Document string `json:"document"`
	SHA256   string `json:"sha256"`
}

type Radio struct {
	Band         string    `json:"band"`
	Frequency    Frequency `json:"frequency,omitempty"`
	BandwidthMHz []int     `json:"bandwidthMHz,omitempty"`
	TXPower      *Value    `json:"txPower,omitempty"`
	AntennaGain  *Value    `json:"antennaGain,omitempty"`
	Evidence     Evidence  `json:"evidence"`
	Confidence   string    `json:"confidence"`
}

type Frequency struct {
	MinMHz int `json:"minMHz"`
	MaxMHz int `json:"maxMHz"`
}

// Meaning distinguishes a measured observation from a configuration or limit.
// Valid meanings are measured, configured, certified_max, regulatory_limit,
// eirp, and conducted_power.
type Value struct {
	Value   float64 `json:"value"`
	Unit    string  `json:"unit"`
	Meaning string  `json:"meaning"`
}

type Evidence struct {
	Document string `json:"document"`
	Page     int    `json:"page"`
	Table    string `json:"table,omitempty"`
}

// DerivedManifest holds constraints that may be consumed by firmware policy.
// It is separate from CertificationRecord so imported observations can never
// silently become device configuration.
type DerivedManifest struct {
	APIVersion   string       `json:"apiVersion"`
	Kind         string       `json:"kind"`
	Device       Device       `json:"device"`
	Jurisdiction string       `json:"jurisdiction"`
	Constraints  []Constraint `json:"constraints"`
}

type Constraint struct {
	Key         string     `json:"key"`
	Value       float64    `json:"value"`
	Unit        string     `json:"unit"`
	Rule        string     `json:"rule"`
	Evidence    []Evidence `json:"evidence"`
	GeneratedBy string     `json:"generatedBy"`
}
