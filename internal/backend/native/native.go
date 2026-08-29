// Package native runs an explicitly configured host-side build command.
package native

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/example/routerctl/internal/backend"
)

// Config configures a local build tool. Command is an executable followed by
// fixed arguments; it is never evaluated by a shell. The requested profile is
// appended as the final argument.
type Config struct {
	Command []string
}

type Backend struct {
	command []string
}

func New(config Config) (*Backend, error) {
	if len(config.Command) == 0 || config.Command[0] == "" {
		return nil, errors.New("native backend: command is required")
	}
	return &Backend{command: append([]string(nil), config.Command...)}, nil
}

func (*Backend) Name() string { return "native" }

func (*Backend) Resolve(context.Context, backend.Request) (backend.Artifact, error) {
	return backend.Artifact{}, errors.New("native backend: resolve is not supported")
}

func (b *Backend) Build(ctx context.Context, req backend.BuildRequest) (backend.BuildResult, error) {
	if req.Device == "" {
		return backend.BuildResult{}, errors.New("native backend: device is required")
	}
	if req.Profile == "" {
		return backend.BuildResult{}, errors.New("native backend: profile is required")
	}
	if !filepath.IsAbs(req.WorkspaceRoot) {
		return backend.BuildResult{}, errors.New("native backend: workspace root must be absolute")
	}
	if !filepath.IsAbs(req.OutputDir) {
		return backend.BuildResult{}, errors.New("native backend: output directory must be absolute")
	}

	args := append(append([]string(nil), b.command[1:]...), req.Profile)
	cmd := exec.CommandContext(ctx, b.command[0], args...)
	cmd.Dir = req.WorkspaceRoot
	cmd.Env = append(os.Environ(),
		"ROUTERCTL_DEVICE="+req.Device,
		"ROUTERCTL_PROFILE="+req.Profile,
		"ROUTERCTL_OUTPUT_DIR="+req.OutputDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return backend.BuildResult{}, fmt.Errorf("native backend: build %q: %w", req.Profile, err)
	}
	return collectArtifacts(req.OutputDir)
}

func collectArtifacts(outputDir string) (backend.BuildResult, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return backend.BuildResult{}, fmt.Errorf("native backend: read output directory: %w", err)
	}
	var artifacts []backend.Artifact
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || entry.Name() == "SHA256SUMS" {
			continue
		}
		path := filepath.Join(outputDir, entry.Name())
		digest, err := fileSHA256(path)
		if err != nil {
			return backend.BuildResult{}, err
		}
		artifacts = append(artifacts, backend.Artifact{Path: path, Digest: "sha256:" + digest})
	}
	if len(artifacts) == 0 {
		return backend.BuildResult{}, fmt.Errorf("native backend: no artifacts in %s", outputDir)
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return backend.BuildResult{Artifacts: artifacts}, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("native backend: open artifact: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("native backend: hash artifact: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
