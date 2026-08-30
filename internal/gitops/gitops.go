// Package gitops provides conservative local Git automation for routerctl.
package gitops

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Status describes the local state of a Git worktree.
type Status struct {
	Root    string   `json:"root"`
	Branch  string   `json:"branch"`
	Changes []string `json:"changes"`
}

// Options controls a commit or sync operation. A sync never force-pushes.
type Options struct {
	Repository string
	Message    string
	DryRun     bool
	Trailers   TrailerOptions
}

// StatusAt returns the current branch and porcelain changes for a repository.
func StatusAt(repository string) (Status, error) {
	root, err := output(repository, "rev-parse", "--show-toplevel")
	if err != nil {
		return Status{}, fmt.Errorf("git status: not a repository: %w", err)
	}
	branch, err := output(root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return Status{}, errors.New("git status: detached HEAD is not supported")
	}
	changes, err := output(root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return Status{}, fmt.Errorf("git status: %w", err)
	}
	return Status{Root: root, Branch: branch, Changes: nonEmptyLines(changes)}, nil
}

// Commit stages all local changes and creates one conventional commit. It never
// amends, resets, or rewrites history.
func Commit(options Options) (string, error) {
	status, err := StatusAt(options.Repository)
	if err != nil {
		return "", err
	}
	if len(status.Changes) == 0 {
		return "", nil
	}
	paths := changedPaths(status.Changes)
	if sensitive := sensitivePaths(paths); len(sensitive) != 0 {
		return "", fmt.Errorf("git commit: refusing to stage possible secret files: %s", strings.Join(sensitive, ", "))
	}
	message := strings.TrimSpace(options.Message)
	if message == "" {
		message = MessageFor(paths)
	}
	if message, err = addRequestedTrailers(message, options.Trailers); err != nil {
		return "", err
	}
	if options.DryRun {
		return message, nil
	}
	if err := run(status.Root, "add", "--all"); err != nil {
		return "", fmt.Errorf("git commit: stage changes: %w", err)
	}
	if err := run(status.Root, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}
	return message, nil
}

// Sync commits local changes, rebases onto the configured upstream, then pushes.
// It deliberately uses ordinary push only; force pushes are never issued.
func Sync(options Options) (string, error) {
	if strings.TrimSpace(options.Message) == "" {
		status, err := StatusAt(options.Repository)
		if err != nil {
			return "", err
		}
		options.Message = MessageFor(changedPaths(status.Changes))
	}
	var err error
	options.Message, err = AddSyncTrailers(options.Message, options.Trailers)
	if err != nil {
		return "", err
	}
	message, err := Commit(options)
	if err != nil {
		return "", err
	}
	status, err := StatusAt(options.Repository)
	if err != nil {
		return "", err
	}
	if options.DryRun {
		return message, nil
	}
	if err := run(status.Root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err != nil {
		return "", errors.New("git sync: current branch has no upstream")
	}
	if err := run(status.Root, "pull", "--rebase"); err != nil {
		return "", fmt.Errorf("git sync: rebase failed; resolve it manually before retrying: %w", err)
	}
	if err := run(status.Root, "push"); err != nil {
		return "", fmt.Errorf("git sync: push failed: %w", err)
	}
	return message, nil
}

// VerifyCommit reads a commit without changing the repository and applies the
// trailer policy to its message and changed paths.
func VerifyCommit(repository, revision string) (CommitVerification, error) {
	root, err := output(repository, "rev-parse", "--show-toplevel")
	if err != nil {
		return CommitVerification{}, fmt.Errorf("git verify commit: not a repository: %w", err)
	}
	if strings.TrimSpace(revision) == "" {
		revision = "HEAD"
	}
	message, err := output(root, "show", "-s", "--format=%B", revision)
	if err != nil {
		return CommitVerification{}, fmt.Errorf("git verify commit: read message: %w", err)
	}
	pathsText, err := output(root, "diff-tree", "--no-commit-id", "--name-only", "-r", revision)
	if err != nil {
		return CommitVerification{}, fmt.Errorf("git verify commit: read changed paths: %w", err)
	}
	return VerifyCommitMessage(nonEmptyLines(pathsText), message), nil
}

func addRequestedTrailers(message string, options TrailerOptions) (string, error) {
	// Sync has already added and validated its complete trailer set before it
	// reaches Commit; do not strip its Generated-By marker.
	if first(parseTrailers(message)[GeneratedByTrailer]) != "" {
		if err := validateTrailerValues(parseTrailers(message)); err != nil {
			return "", err
		}
		return message, nil
	}
	if !options.AIAssisted && strings.TrimSpace(options.ReviewedBy) == "" && strings.TrimSpace(options.AutomationActor) == "" {
		return message, nil
	}
	// Commit has no generator trailer. Reuse the strict value validation and
	// remove the sync marker it adds for this non-sync operation.
	withSync, err := AddSyncTrailers(message, options)
	if err != nil {
		return "", err
	}
	return strings.Replace(withSync, "\n\n"+GeneratedByTrailer+": "+routerctlSync, "", 1), nil
}

// MessageFor returns a deterministic conventional-commit message from paths.
func MessageFor(paths []string) string {
	if len(paths) == 0 {
		return "chore: update workspace"
	}
	paths = append([]string(nil), paths...)
	sort.Strings(paths)
	kind, scope := "chore", ""
	for _, path := range paths {
		slashPath := filepath.ToSlash(path)
		switch {
		case strings.HasPrefix(slashPath, ".github/"):
			kind, scope = "ci", ""
		case strings.Contains(slashPath, "test"):
			if kind == "chore" {
				kind, scope = "test", ""
			}
		case strings.HasSuffix(slashPath, ".md"):
			if kind == "chore" {
				kind, scope = "docs", ""
			}
		case strings.HasPrefix(slashPath, "internal/regulatory/"):
			if kind == "chore" {
				kind, scope = "fix", "regulatory"
			}
		case strings.HasPrefix(slashPath, "internal/") || strings.HasPrefix(slashPath, "cmd/"):
			if kind == "chore" {
				kind, scope = "feat", "routerctl"
			}
		}
	}
	if scope != "" {
		return fmt.Sprintf("%s(%s): update %d files", kind, scope, len(paths))
	}
	return fmt.Sprintf("%s: update %d files", kind, len(paths))
}

func changedPaths(changes []string) []string {
	paths := make([]string, 0, len(changes))
	for _, change := range changes {
		if len(change) < 4 {
			continue
		}
		path := strings.TrimSpace(change[3:])
		if before, after, ok := strings.Cut(path, " -> "); ok {
			path = after
			_ = before
		}
		paths = append(paths, path)
	}
	return paths
}

func sensitivePaths(paths []string) []string {
	var found []string
	for _, path := range paths {
		base := strings.ToLower(filepath.Base(path))
		if base == ".env" || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") || strings.Contains(base, "credential") || strings.Contains(base, "secret") {
			found = append(found, path)
		}
	}
	return found
}

func output(dir string, args ...string) (string, error) {
	var stdout bytes.Buffer
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Stdout = &stdout
	if err := command.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func run(dir string, args ...string) error {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	return command.Run()
}

func nonEmptyLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
