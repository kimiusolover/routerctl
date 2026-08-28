// Package validate checks that imported certification observations have enough
// context to be reviewed before any firmware constraint is derived.
package validate

import (
	"fmt"
	"strings"

	"github.com/example/routerctl/internal/regulatory/model"
)

var meanings = map[string]bool{
	model.MeaningMeasured: true, model.MeaningConfigured: true,
	model.MeaningCertifiedMax: true, model.MeaningRegulatoryLimit: true,
	model.MeaningEIRP: true, model.MeaningConductedPower: true,
}

func Record(record *model.CertificationRecord) error {
	if record == nil || record.APIVersion != model.APIVersion || record.Kind != "CertificationRecord" {
		return fmt.Errorf("regulatory record: unsupported record")
	}
	if record.Jurisdiction == "" || record.Certification.Authority == "" || record.Certification.Number == "" {
		return fmt.Errorf("regulatory record: jurisdiction and certification authority/number are required")
	}
	if record.Certification.Source.Document == "" || len(record.Certification.Source.SHA256) != 64 {
		return fmt.Errorf("regulatory record: source document and SHA-256 are required")
	}
	if len(record.Radios) == 0 {
		return fmt.Errorf("regulatory record: no radios")
	}
	for i, radio := range record.Radios {
		if radio.Band == "" || radio.Frequency.MinMHz <= 0 || radio.Frequency.MaxMHz < radio.Frequency.MinMHz {
			return fmt.Errorf("regulatory record: radio %d has invalid frequency", i)
		}
		if radio.Evidence.Document == "" || radio.Evidence.Page <= 0 {
			return fmt.Errorf("regulatory record: radio %d lacks page evidence", i)
		}
		for _, value := range []*model.Value{radio.TXPower, radio.AntennaGain} {
			if value != nil && (!meanings[value.Meaning] || strings.TrimSpace(value.Unit) == "") {
				return fmt.Errorf("regulatory record: radio %d has invalid value meaning or unit", i)
			}
		}
	}
	return nil
}
