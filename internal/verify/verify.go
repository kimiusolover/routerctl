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
	if m.Spec.Backend == "native" {
		if m.Spec.Build.Profile == "" || m.Spec.Build.Repository == "" || len(m.Spec.Build.Command) == 0 || m.Spec.Build.Output == "" {
			errs = append(errs, "spec.build.profile, repository, command, and output are required for native backend")
		}
		if m.Spec.Build.Profile != "" && m.Spec.Build.Profile != m.Spec.Device {
			errs = append(errs, "spec.build.profile must equal spec.device for native backend")
		}
		if m.Spec.Artifact.Expected.Device == "" || m.Spec.Artifact.Expected.Format == "" || m.Spec.Artifact.Expected.MaxSizeBytes <= 0 {
			errs = append(errs, "spec.artifact.expected.device, format, and max_size_bytes are required for native backend")
		}
		if m.Spec.Artifact.Expected.Device != "" && m.Spec.Artifact.Expected.Device != m.Spec.Device {
			errs = append(errs, "spec.artifact.expected.device must equal spec.device for native backend")
		}
	}
	if m.Spec.Transport == "" {
		errs = append(errs, "spec.transport is required")
	}
	return Result{OK: len(errs) == 0, Errors: errs}
}
