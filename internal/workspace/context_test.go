package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveSeparatesWorkspaceGitAndComponentRoots(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, markerPath), "{}")
	mustWrite(t, filepath.Join(root, registryPath), `{"components":[{"name":"firmware","root":"vendor/firmware"}]}`)
	file := filepath.Join(root, "vendor/firmware/config/device.yaml")
	mustWrite(t, file, "device: test")
	if err := runGit(root, "init"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	componentRoot := filepath.Join(root, "vendor/firmware")
	if err := runGit(componentRoot, "init"); err != nil {
		t.Fatal(err)
	}

	ctx, err := Resolve(filepath.Join(root, "vendor/firmware/config"), "device.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.WorkspaceRoot != root || ctx.ComponentRoot != componentRoot || ctx.GitRoot != componentRoot {
		t.Fatalf("wrong roots: %+v", ctx)
	}
	if ctx.Component != "firmware" || ctx.ComponentPath != filepath.Join("config", "device.yaml") {
		t.Fatalf("wrong component context: %+v", ctx)
	}
}

func TestResolveRejectsUnregisteredFile(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, markerPath), "{}")
	mustWrite(t, filepath.Join(root, registryPath), `{"components":[{"name":"firmware","root":"firmware"}]}`)
	file := filepath.Join(root, "notes.txt")
	mustWrite(t, file, "no")
	if _, err := Resolve(root, "notes.txt"); err == nil {
		t.Fatal("Resolve succeeded for unregistered file")
	}
}

func TestResolveDefaultsComponentToWorkspaceWithoutRegistry(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, markerPath), "{}")
	mustWrite(t, filepath.Join(root, "nested/device.yaml"), "device: test")
	ctx, err := Resolve(filepath.Join(root, "nested"), "device.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.ComponentRoot != root || ctx.ComponentPath != filepath.Join("nested", "device.yaml") {
		t.Fatalf("wrong default context: %+v", ctx)
	}
}

func mustWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runGit(dir string, args ...string) error {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	return command.Run()
}
