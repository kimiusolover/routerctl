package verify

import "github.com/example/routerctl/internal/manifest"

type Result struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors,omitempty"`
}

func Manifest(m manifest.Manifest) Result {
	var errs []string
	if m.APIVersion != "routerctl.dev/v1alpha1" {
		errs = append(errs, "unsupported apiVersion")
	}
	if m.Kind != "Device" {
		errs = append(errs, "kind must be Device")
	}
	if m.Metadata.Name == "" {
		errs = append(errs, "metadata.name is required")
	}
	if m.Spec.Device == "" {
		errs = append(errs, "spec.device is required")
	}
	if m.Spec.Backend == "" {
		errs = append(errs, "spec.backend is required")
	}
	if m.Spec.Backend == "github" && m.Spec.GitHub.Repository == "" {
		errs = append(errs, "spec.github.repository is required for github backend")
	}
	if m.Spec.Transport == "" {
		errs = append(errs, "spec.transport is required")
	}
	return Result{OK: len(errs) == 0, Errors: errs}
}
