package planner

import (
	"fmt"

	"github.com/example/routerctl/internal/manifest"
)

type Plan struct {
	Device    string   `json:"device"`
	Backend   string   `json:"backend"`
	Transport string   `json:"transport"`
	Target    string   `json:"target"`
	Steps     []string `json:"steps"`
}

func Create(m manifest.Manifest) (Plan, error) {
	if m.Spec.Device == "" {
		return Plan{}, fmt.Errorf("device is required")
	}
	return Plan{
		Device:    m.Spec.Device,
		Backend:   m.Spec.Backend,
		Transport: m.Spec.Transport,
		Target:    m.Spec.Target,
		Steps: []string{
			"validate-manifest",
			"resolve-backend",
			"resolve-artifact",
			"resolve-transport",
			"verify-target",
		},
	}, nil
}
