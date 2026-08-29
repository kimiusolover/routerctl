package manifest

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Manifest intentionally starts small. The loader supports the simple YAML
// subset used by the bootstrap manifests without introducing dependencies.
type Manifest struct {
	APIVersion string   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string   `json:"kind" yaml:"kind"`
	Metadata   Metadata `json:"metadata" yaml:"metadata"`
	Spec       Spec     `json:"spec" yaml:"spec"`
}

type Metadata struct {
	Name string `json:"name" yaml:"name"`
}

type Spec struct {
	Device    string   `json:"device" yaml:"device"`
	Backend   string   `json:"backend" yaml:"backend"`
	Transport string   `json:"transport" yaml:"transport"`
	Target    string   `json:"target" yaml:"target"`
	GitHub    GitHub   `json:"github" yaml:"github"`
	Build     Build    `json:"build" yaml:"build"`
	Artifact  Artifact `json:"artifact" yaml:"artifact"`
}

type Build struct {
	Profile    string   `json:"profile" yaml:"profile"`
	Repository string   `json:"repository" yaml:"repository"`
	Command    []string `json:"command" yaml:"command"`
	Output     string   `json:"output" yaml:"output"`
}

type Artifact struct {
	Expected ArtifactExpected `json:"expected" yaml:"expected"`
}

type ArtifactExpected struct {
	Device       string `json:"device" yaml:"device"`
	Format       string `json:"format" yaml:"format"`
	MaxSizeBytes int64  `json:"max_size_bytes" yaml:"max_size_bytes"`
	BoardID      string `json:"board_id" yaml:"board_id"`
}

// GitHub configures the GitHub Releases backend. Tokens are never stored in a
// manifest; the CLI reads GITHUB_TOKEN when it resolves a private release.
type GitHub struct {
	Repository string `json:"repository" yaml:"repository"`
	Tag        string `json:"tag" yaml:"tag"`
}

func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest: parse YAML: %w", err)
	}
	return m, nil
}
