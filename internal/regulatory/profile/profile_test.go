package profile

import "testing"

func valid() *Profile {
	return &Profile{APIVersion: APIVersion, Kind: "CertificationProfile", Metadata: Metadata{ID: "jp-test", EvidenceStatus: "verified"}, Subject: Subject{Jurisdiction: "JP", Authority: "MIC", Certification: Certification{Scheme: "construction-design-certification", Number: "201-test"}, Hardware: Hardware{Manufacturer: "TP-Link", Model: "AX23V", Revision: "1.0", CalibrationRequired: true}}, Constraints: Constraints{Spectrum: map[string]Band{"w53": {LegalRangeMHz: []float64{5250, 5350}, PrimaryChannels: []int{52}, WidthsMHz: []int{20}, DFSRequired: true, Power: Power{Regulatory: map[int]float64{20: 10}, CertifiedRated: map[int]float64{20: 6}}}}, DFS: DFS{CACMinSeconds: 60, ChannelMoveMaxSeconds: 10, NonOccupancyMinSeconds: 1800}, Enforcement: Enforcement{Default: "deny"}}}
}
func TestTXIsDeniedUntilEvidenceIsVerified(t *testing.T) {
	p := valid()
	p.Metadata.EvidenceStatus = "incomplete"
	if p.TXPermitted() {
		t.Fatal("unverified profile permitted TX")
	}
}
func TestRejectsMissingCertifiedLimit(t *testing.T) {
	p := valid()
	p.Constraints.Spectrum["w53"] = Band{LegalRangeMHz: []float64{5250, 5350}, PrimaryChannels: []int{52}, WidthsMHz: []int{20}, Power: Power{Regulatory: map[int]float64{20: 10}}}
	if Validate(p) == nil {
		t.Fatal("accepted a missing certified limit")
	}
}
