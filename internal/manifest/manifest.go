package manifest

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Manifest intentionally starts small. The loader supports the simple YAML
// subset used by the bootstrap manifests without introducing dependencies.
type Manifest struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`
}

type Metadata struct {
	Name string `json:"name"`
}

type Spec struct {
	Device    string `json:"device"`
	Backend   string `json:"backend"`
	Transport string `json:"transport"`
	Target    string `json:"target"`
	GitHub    GitHub `json:"github"`
}

// GitHub configures the GitHub Releases backend. Tokens are never stored in a
// manifest; the CLI reads GITHUB_TOKEN when it resolves a private release.
type GitHub struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

func Load(path string) (Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer f.Close()

	var m Manifest
	section := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, ":") {
			section = strings.TrimSuffix(line, ":")
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return Manifest{}, fmt.Errorf("unsupported manifest line: %q", line)
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

		switch section {
		case "":
			switch key {
			case "apiVersion":
				m.APIVersion = val
			case "kind":
				m.Kind = val
			}
		case "metadata":
			if key == "name" {
				m.Metadata.Name = val
			}
		case "spec":
			switch key {
			case "device":
				m.Spec.Device = val
			case "backend":
				m.Spec.Backend = val
			case "transport":
				m.Spec.Transport = val
			case "target":
				m.Spec.Target = val
			}
		case "github":
			switch key {
			case "repository":
				m.Spec.GitHub.Repository = val
			case "tag":
				m.Spec.GitHub.Tag = val
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Manifest{}, err
	}
	if m.APIVersion == "" {
		return Manifest{}, errors.New("manifest: apiVersion is required")
	}
	return m, nil
}
