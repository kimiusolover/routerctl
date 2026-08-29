package label

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Layout is a reviewed, device-specific fixed-coordinate crop definition.
// It is intentionally limited to allowlisted roles, so a layout cannot request
// an output containing credentials, serial numbers, MAC addresses, or QR data.
type Layout struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Device struct {
		Vendor   string `yaml:"vendor"`
		Model    string `yaml:"model"`
		Revision string `yaml:"revision"`
	} `yaml:"device"`
	Crops map[string]struct {
		Geometry string `yaml:"geometry"`
	} `yaml:"crops"`
}

func LoadLayout(path string) (Layout, []CropTarget, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Layout{}, nil, fmt.Errorf("label layout: read: %w", err)
	}
	var layout Layout
	if err := yaml.Unmarshal(data, &layout); err != nil {
		return Layout{}, nil, fmt.Errorf("label layout: parse YAML: %w", err)
	}
	if layout.APIVersion != "routerctl.regulatory/v1" || layout.Kind != "LabelLayout" || !layoutIDRegex.MatchString(layout.Metadata.Name) {
		return Layout{}, nil, fmt.Errorf("label layout: apiVersion, kind, and metadata.name (vendor/model/revision-version) are required")
	}
	if len(layout.Crops) != len(RequiredRoles) {
		return Layout{}, nil, fmt.Errorf("label layout: exactly these crops are required: %v", RequiredRoles)
	}
	crops := make([]CropTarget, 0, len(layout.Crops))
	for _, role := range RequiredRoles {
		crop, ok := layout.Crops[role]
		if !ok || crop.Geometry == "" {
			return Layout{}, nil, fmt.Errorf("label layout: crop %q is required", role)
		}
		crops = append(crops, CropTarget{Role: role, Geometry: crop.Geometry})
	}
	for role := range layout.Crops {
		if !AllowedRoles[role] {
			return Layout{}, nil, fmt.Errorf("label layout: unauthorized crop role %q", role)
		}
	}
	return layout, crops, nil
}
