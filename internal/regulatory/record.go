package regulatory

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/example/routerctl/internal/regulatory/model"
)

func ReadRecord(path string) (*model.CertificationRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var record model.CertificationRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("regulatory: parse certification record: %w", err)
	}
	return &record, nil
}

func ReadDerived(path string) (*model.DerivedManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest model.DerivedManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("regulatory: parse derived manifest: %w", err)
	}
	return &manifest, nil
}
