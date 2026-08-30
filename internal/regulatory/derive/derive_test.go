package derive

import (
	"strings"
	"testing"

	"github.com/example/routerctl/internal/regulatory/model"
)

const sourceSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func record(meaning string) *model.CertificationRecord {
	return &model.CertificationRecord{
		APIVersion: model.APIVersion, Kind: "CertificationRecord", Jurisdiction: "JP",
		Device:        model.Device{Vendor: "TP-Link", Model: "Archer AX23V", Revision: "v1"},
		Certification: model.Certification{Authority: "MIC", Number: "201-test", Source: model.Source{Document: "report.pdf", SHA256: sourceSHA}},
		Radios:        []model.Radio{{Band: "5GHz", Frequency: model.Frequency{MinMHz: 5180, MaxMHz: 5240}, TXPower: &model.Value{Value: 17, Unit: "dBm", Meaning: meaning}, Evidence: model.Evidence{Document: "report.pdf", Page: 23, Table: "RF Output Power"}, Confidence: "verified"}},
	}
}

func TestConstraintsUsesOnlyExplicitCertifiedMaximum(t *testing.T) {
	derived, err := Constraints(record(model.MeaningCertifiedMax))
	if err != nil {
		t.Fatal(err)
	}
	if len(derived.Constraints) != 1 || derived.Constraints[0].Key != "wifi.5GHz.max_tx_power_dbm" || derived.Constraints[0].Value != 17 {
		t.Fatalf("constraints = %#v", derived.Constraints)
	}
}

func TestConstraintsRefusesConductedPowerInference(t *testing.T) {
	_, err := Constraints(record(model.MeaningConductedPower))
	if err == nil || !strings.Contains(err.Error(), "refusing to infer") {
		t.Fatalf("error = %v", err)
	}
}
