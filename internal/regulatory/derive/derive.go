// Package derive creates conservative firmware constraints from explicit,
// evidence-backed certification limits.
package derive

import (
	"fmt"
	"sort"

	"github.com/example/routerctl/internal/regulatory/model"
	"github.com/example/routerctl/internal/regulatory/validate"
)

func Constraints(record *model.CertificationRecord) (*model.DerivedManifest, error) {
	if err := validate.Record(record); err != nil {
		return nil, err
	}
	result := &model.DerivedManifest{APIVersion: model.APIVersion, Kind: "DerivedManifest", Device: record.Device, Jurisdiction: record.Jurisdiction}
	for _, radio := range record.Radios {
		// Conducted, measured, configured, and EIRP observations require an
		// explicit policy rule; none is assumed here.
		if radio.TXPower == nil || radio.TXPower.Meaning != model.MeaningCertifiedMax || radio.TXPower.Unit != "dBm" {
			continue
		}
		result.Constraints = append(result.Constraints, model.Constraint{
			Key: "wifi." + radio.Band + ".max_tx_power_dbm", Value: radio.TXPower.Value, Unit: "dBm",
			Rule:     "explicit certified maximum from certification record",
			Evidence: []model.Evidence{radio.Evidence}, GeneratedBy: "routerctl-regulatory-v0.1",
		})
	}
	if len(result.Constraints) == 0 {
		return nil, fmt.Errorf("regulatory derive: no explicit certified_max observations; refusing to infer firmware limits")
	}
	sort.Slice(result.Constraints, func(i, j int) bool { return result.Constraints[i].Key < result.Constraints[j].Key })
	return result, nil
}
