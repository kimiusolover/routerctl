package gitops

import (
	"strings"
	"testing"
)

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

func TestVerifyCommitMessage(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		message  string
		ok       bool
		warnings int
	}{
		{"ordinary generation", []string{"docs/note.md"}, "docs: update\n\nGenerated-By: routerctl sync", true, 1},
		{"regulated change requires reviewer", []string{"examples/ax23v/regulatory/JP/certification-profile.yaml"}, "fix: regulatory", false, 0},
		{"reviewed release is valid", []string{".github/workflows/release.yml"}, "ci: publish\n\nReviewed-By: Yuta Nakano", true, 0},
		{"bad ai identity fails", []string{"README.md"}, "docs: update\n\nAI-Assisted-By: other", false, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := VerifyCommitMessage(test.paths, test.message)
			if got.OK() != test.ok {
				t.Fatalf("OK=%v errors=%v", got.OK(), got.Errors)
			}
			if len(got.Warnings) != test.warnings {
				t.Fatalf("warnings=%v", got.Warnings)
			}
		})
	}
}

func TestAddSyncTrailers(t *testing.T) {
	got, err := AddSyncTrailers("chore: sync", TrailerOptions{AIAssisted: true, ReviewedBy: "Yuta Nakano", AutomationActor: "router-os-bot[bot]"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"AI-Assisted-By: OpenAI ChatGPT", "Generated-By: routerctl sync", "Reviewed-By: Yuta Nakano", "Automation-Actor: router-os-bot[bot]"} {
		if !strings.Contains(got, want) {
			t.Errorf("message missing %q: %s", want, got)
		}
	}
}
