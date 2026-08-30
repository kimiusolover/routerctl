package native

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/example/routerctl/internal/backend"
)

func TestBuildRunsConfiguredCommandAndCollectsArtifacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test command uses POSIX sh")
	}
	root := t.TempDir()
	out := filepath.Join(root, "dist")
	if err := os.Mkdir(out, 0o700); err != nil {
		t.Fatal(err)
	}
	b, err := New(Config{Command: []string{"sh", "-c", "printf '%s' \"$ROUTERCTL_PROFILE\" > \"$ROUTERCTL_OUTPUT_DIR/firmware.bin\""}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := b.Build(context.Background(), backend.BuildRequest{Device: "ax23v", Profile: "release", WorkspaceRoot: root, OutputDir: out})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].Digest != "sha256:a4d451ec23463726f72c43d64c710968f6b602cd653b4de8adee1b556240a829" {
		t.Fatalf("artifacts = %#v", result.Artifacts)
	}
}

func TestBuildRequiresAbsoluteRoots(t *testing.T) {
	b, err := New(Config{Command: []string{"true"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Build(context.Background(), backend.BuildRequest{Device: "ax23v", Profile: "release", WorkspaceRoot: ".", OutputDir: "/tmp"})
	if err == nil {
		t.Fatal("Build succeeded with a relative workspace root")
	}
}
