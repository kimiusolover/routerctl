package explain

import (
	"strings"
	"testing"

	"github.com/example/routerctl/internal/regulatory/model"
)

func TestConstraintPrintsEvidence(t *testing.T) {
	manifest := &model.DerivedManifest{Kind: "DerivedManifest", Constraints: []model.Constraint{{Key: "wifi.5GHz.max_tx_power_dbm", Value: 17, Unit: "dBm", Rule: "test rule", GeneratedBy: "test", Evidence: []model.Evidence{{Document: "report.pdf", Page: 23, Table: "RF Output Power"}}}}}
	text, err := Constraint(manifest, "wifi.5GHz.max_tx_power_dbm")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "17 dBm") || !strings.Contains(text, "report.pdf page 23") {
		t.Fatalf("text = %q", text)
	}
}
