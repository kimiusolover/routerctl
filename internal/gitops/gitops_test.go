package gitops

import "testing"

func TestMessageFor(t *testing.T) {
	tests := []struct {
		paths []string
		want  string
	}{
		{[]string{".github/workflows/ci.yml"}, "ci: update 1 files"},
		{[]string{"README.md"}, "docs: update 1 files"},
		{[]string{"internal/regulatory/derive/derive.go"}, "fix(regulatory): update 1 files"},
		{[]string{"cmd/routerctl/main.go", "internal/cli/cli.go"}, "feat(routerctl): update 2 files"},
	}
	for _, test := range tests {
		if got := MessageFor(test.paths); got != test.want {
			t.Errorf("MessageFor(%v) = %q, want %q", test.paths, got, test.want)
		}
	}
}

func TestChangedPathsAndSensitivePaths(t *testing.T) {
	paths := changedPaths([]string{" M README.md", "?? config/.env", "R  old.txt -> keys/release.pem"})
	if got, want := len(paths), 3; got != want {
		t.Fatalf("paths = %v", paths)
	}
	if got := sensitivePaths(paths); len(got) != 2 {
		t.Errorf("sensitive paths = %v, want 2", got)
	}
}
