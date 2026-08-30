package mic

import (
	"testing"

	"github.com/example/routerctl/internal/regulatory/importer"
	"github.com/example/routerctl/internal/regulatory/model"
)

func TestExtractPreservesCertifiedMeaningAndEvidence(t *testing.T) {
	doc := importer.Document{Name: "report.txt", SHA256: "abc", Text: "MIC 技術基準適合\nVendor: TP-Link\nModel: Archer AX23V\nRevision: v1\n認証番号: 201-AX23V\nPage 17\n2412 MHz - 2472 MHz\nBandwidth: 20, 40 MHz\nCertified Maximum Conducted Output Power: 17.84 dBm\nAntenna Gain: 3.0 dBi"}
	record, err := (Importer{}).Extract(doc)
	if err != nil {
		t.Fatal(err)
	}
	radio := record.Radios[0]
	if record.Jurisdiction != "JP" || record.Certification.Number != "201-AX23V" || radio.Band != "2.4GHz" {
		t.Fatalf("record = %#v", record)
	}
	if record.Device.Vendor != "TP-Link" || record.Device.Model != "Archer AX23V" || record.Device.Revision != "v1" {
		t.Fatalf("device = %#v", record.Device)
	}
	if radio.TXPower == nil || radio.TXPower.Meaning != "conducted_power" || radio.TXPower.Value != 17.84 {
		t.Fatalf("power = %#v", radio.TXPower)
	}
	if radio.Evidence.Page != 17 || radio.Evidence.Document != "report.txt" || record.Certification.Source.SHA256 != "abc" {
		t.Fatalf("evidence = %#v, source = %#v", radio.Evidence, record.Certification.Source)
	}
}

func TestMatchesDeviceRejectsMismatch(t *testing.T) {
	record := &model.CertificationRecord{Device: model.Device{Vendor: "TP-Link", Model: "Archer AX23V", Revision: "v1"}}
	if err := MatchesDevice(record, model.Device{Vendor: "TP-Link", Model: "Archer AX55", Revision: "v1"}); err == nil {
		t.Fatal("MatchesDevice accepted a different model")
	}
}

func TestExtractSeriesModelAndObservedVendorWithoutRevision(t *testing.T) {
	doc := importer.Document{Name: "report.txt", SHA256: "abc", Text: "MIC 技術基準適合\nApplicant: TP-Link Corporation Limited\nManufacturer: TP-Link Corporation Limited\nTest Model: Archer AX1800\nSeries Model: Archer AX23V\n2412 MHz - 2472 MHz"}
	record, err := (Importer{}).ExtractWithNumber(doc, "201-AX23V", model.NumberSource{MatchStatus: "unconfirmed"})
	if err != nil {
		t.Fatal(err)
	}
	if record.Device.Model != "Archer AX23V" || record.Device.Revision != "" {
		t.Fatalf("device = %#v", record.Device)
	}
	if len(record.Device.VendorCandidates) != 2 || len(record.Device.ModelCandidates) != 1 {
		t.Fatalf("observations = %#v", record.Device)
	}
	if err := MatchesDevice(record, model.Device{Vendor: "TP-Link", Model: "Archer AX23V", Revision: "v1"}); err != nil {
		t.Fatalf("expected series model with an unknown revision to remain importable: %v", err)
	}
	if record.Certification.NumberSource.MatchStatus != "unconfirmed" {
		t.Fatalf("match status = %q", record.Certification.NumberSource.MatchStatus)
	}
}

func TestMatchesDeviceRejectsObservedRevisionMismatch(t *testing.T) {
	record := &model.CertificationRecord{Device: model.Device{Vendor: "TP-Link", Model: "Archer AX23V", Revision: "v2"}}
	if err := MatchesDevice(record, model.Device{Vendor: "TP-Link", Model: "Archer AX23V", Revision: "v1"}); err == nil {
		t.Fatal("MatchesDevice accepted a different observed revision")
	}
}

func TestSameVendorAllowsOnlyLegalEntitySuffixes(t *testing.T) {
	if !sameVendor("TP-Link Corporation Limited", "TP-Link") {
		t.Fatal("legal entity suffixes should be accepted")
	}
	if sameVendor("TP-Link Systems", "TP-Link") {
		t.Fatal("shared brand words must not be accepted as a vendor match")
	}
}
