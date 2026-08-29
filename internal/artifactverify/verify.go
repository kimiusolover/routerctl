// Package artifactverify validates a built image against its router-firmware
// artifact manifest before routerctl reports it as usable.
package artifactverify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/example/routerctl/internal/backend"
	"github.com/example/routerctl/internal/manifest"
)

type buildManifest struct {
	Schema    int    `json:"schema"`
	Device    string `json:"device"`
	Flashable bool   `json:"flashable"`
	Artifacts []struct {
		Name    string `json:"name"`
		SHA256  string `json:"sha256"`
		Size    int64  `json:"size"`
		Format  string `json:"format"`
		BoardID string `json:"board_id"`
	} `json:"artifacts"`
}

// Verify requires a companion <device>.manifest.json. The manifest is the
// producer's assertion of device identity and image format; the digest and
// size are independently compared with routerctl's observed artifact.
func Verify(output string, artifact backend.Artifact, expected manifest.ArtifactExpected) error {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(output), expected.Device+".manifest.json"))
	if err != nil {
		return fmt.Errorf("artifact verification: read artifact manifest: %w", err)
	}
	var m buildManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("artifact verification: parse artifact manifest: %w", err)
	}
	if m.Device != expected.Device {
		return fmt.Errorf("artifact verification: manifest device %q does not match %q", m.Device, expected.Device)
	}
	if m.Schema != 2 {
		return fmt.Errorf("artifact verification: unsupported manifest schema %d", m.Schema)
	}
	info, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("artifact verification: stat artifact: %w", err)
	}
	if info.Size() > expected.MaxSizeBytes {
		return fmt.Errorf("artifact verification: artifact is %d bytes; maximum is %d", info.Size(), expected.MaxSizeBytes)
	}
	for _, entry := range m.Artifacts {
		if entry.Name == filepath.Base(output) {
			if entry.Format != expected.Format {
				return fmt.Errorf("artifact verification: manifest format %q does not match %q", entry.Format, expected.Format)
			}
			if expected.BoardID != "" && entry.BoardID != expected.BoardID {
				return fmt.Errorf("artifact verification: manifest board_id %q does not match %q", entry.BoardID, expected.BoardID)
			}
			if !m.Flashable || entry.Format == "router-firmware-unflashable-fixture" {
				return fmt.Errorf("artifact verification: unflashable artifact cannot satisfy build output")
			}
			if entry.Size != info.Size() {
				return fmt.Errorf("artifact verification: manifest size does not match artifact")
			}
			if "sha256:"+entry.SHA256 != artifact.Digest {
				return fmt.Errorf("artifact verification: manifest SHA-256 does not match artifact")
			}
			return nil
		}
	}
	return fmt.Errorf("artifact verification: output %q is absent from artifact manifest", filepath.Base(output))
}
