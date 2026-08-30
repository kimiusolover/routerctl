// Package firmwaregate verifies the router-firmware safety declaration before
// routerctl delegates a native image build to it.
package firmwaregate

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type deviceDefinition struct {
	ID         string `yaml:"id"`
	Status     string `yaml:"status"`
	Partitions string `yaml:"partitions"`
}

type partitionDefinition struct {
	Status string `yaml:"status"`
}

// Check rejects image assembly unless the selected device is explicitly
// supported and its referenced partition map is explicitly verified.
func Check(repository, device string) error {
	path := filepath.Join(repository, "devices", device, "device.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("firmware gate: read device definition: %w", err)
	}
	var definition deviceDefinition
	if err := yaml.Unmarshal(data, &definition); err != nil {
		return fmt.Errorf("firmware gate: parse device definition: %w", err)
	}
	if definition.ID != device {
		return fmt.Errorf("firmware gate: device definition id %q does not match %q", definition.ID, device)
	}
	if definition.Status != "supported" {
		return fmt.Errorf("firmware gate: device %q is %q, not supported", device, definition.Status)
	}
	if definition.Partitions == "" || filepath.IsAbs(definition.Partitions) {
		return fmt.Errorf("firmware gate: invalid partition definition for %q", device)
	}
	partitionsPath := filepath.Join(filepath.Dir(path), definition.Partitions)
	data, err = os.ReadFile(partitionsPath)
	if err != nil {
		return fmt.Errorf("firmware gate: read partition definition: %w", err)
	}
	var partitions partitionDefinition
	if err := yaml.Unmarshal(data, &partitions); err != nil {
		return fmt.Errorf("firmware gate: parse partition definition: %w", err)
	}
	if partitions.Status != "verified" {
		return fmt.Errorf("firmware gate: partition map for %q is %q, not verified", device, partitions.Status)
	}
	return nil
}
