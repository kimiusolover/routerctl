// Package workspace resolves routerctl workspace and component context without
// assuming that a Git checkout, workspace, component, or current directory are
// the same directory.
package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	markerPath   = ".routerctl/workspace.json"
	registryPath = ".routerctl/components.json"
)

// Context is the fully resolved location information for one file.
// Empty GitRoot and SuperprojectRoot mean that the file is not in a Git
// repository, or that the repository is not a submodule, respectively.
type Context struct {
	CurrentDirectory string
	FilePath         string
	WorkspaceRoot    string
	GitRoot          string
	SuperprojectRoot string
	Component        string
	ComponentRoot    string
	ComponentPath    string
}

type registry struct {
	Components []component `json:"components"`
}

type component struct {
	Name string `json:"name"`
	Root string `json:"root"`
}

// Resolve normalizes path relative to cwd and resolves each root in a fixed
// order. The workspace is identified only by its marker, never by Git.
func Resolve(cwd, path string) (Context, error) {
	currentDirectory, err := absoluteClean(cwd, "cwd")
	if err != nil {
		return Context{}, err
	}
	filePath, err := normalizePath(currentDirectory, path)
	if err != nil {
		return Context{}, err
	}

	workspaceRoot, err := findWorkspaceRoot(filepath.Dir(filePath))
	if err != nil {
		return Context{}, err
	}
	gitRoot, superprojectRoot := gitRoots(filePath)

	name, componentRoot, err := resolveComponent(workspaceRoot, filePath)
	if err != nil {
		return Context{}, err
	}
	componentPath, err := relativeInside(componentRoot, filePath)
	if err != nil {
		return Context{}, fmt.Errorf("workspace: file is outside component root: %w", err)
	}

	return Context{
		CurrentDirectory: currentDirectory,
		FilePath:         filePath,
		WorkspaceRoot:    workspaceRoot,
		GitRoot:          gitRoot,
		SuperprojectRoot: superprojectRoot,
		Component:        name,
		ComponentRoot:    componentRoot,
		ComponentPath:    componentPath,
	}, nil
}

func absoluteClean(path, label string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("workspace: %s is required", label)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("workspace: normalize %s: %w", label, err)
	}
	return filepath.Clean(abs), nil
}

func normalizePath(cwd, path string) (string, error) {
	if path == "" {
		return "", errors.New("workspace: filepath is required")
	}
	if filepath.IsAbs(path) {
		return absoluteClean(path, "filepath")
	}
	return absoluteClean(filepath.Join(cwd, path), "filepath")
}

func findWorkspaceRoot(start string) (string, error) {
	for dir := start; ; dir = filepath.Dir(dir) {
		if info, err := os.Stat(filepath.Join(dir, markerPath)); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("workspace: %s not found above %s", markerPath, start)
}

func gitRoots(filePath string) (string, string) {
	gitRoot := gitOutput(filepath.Dir(filePath), "rev-parse", "--show-toplevel")
	if gitRoot == "" {
		return "", ""
	}
	return gitRoot, gitOutput(gitRoot, "rev-parse", "--show-superproject-working-tree")
}

func gitOutput(dir string, args ...string) string {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func resolveComponent(workspaceRoot, filePath string) (string, string, error) {
	registryFile := filepath.Join(workspaceRoot, registryPath)
	data, err := os.ReadFile(registryFile)
	if errors.Is(err, os.ErrNotExist) {
		return "workspace", workspaceRoot, nil
	}
	if err != nil {
		return "", "", fmt.Errorf("workspace: read component registry: %w", err)
	}
	var r registry
	if err := json.Unmarshal(data, &r); err != nil {
		return "", "", fmt.Errorf("workspace: parse component registry: %w", err)
	}

	bestName, bestRoot := "", ""
	for _, entry := range r.Components {
		if entry.Name == "" || entry.Root == "" || filepath.IsAbs(entry.Root) {
			return "", "", fmt.Errorf("workspace: invalid component registry entry %+v", entry)
		}
		root := filepath.Clean(filepath.Join(workspaceRoot, entry.Root))
		if _, err := relativeInside(workspaceRoot, root); err != nil {
			return "", "", fmt.Errorf("workspace: component %q escapes workspace", entry.Name)
		}
		if _, err := relativeInside(root, filePath); err == nil && len(root) > len(bestRoot) {
			bestName, bestRoot = entry.Name, root
		}
	}
	if bestRoot == "" {
		return "", "", fmt.Errorf("workspace: no registered component contains %s", filePath)
	}
	return bestName, bestRoot, nil
}

// relativeInside returns a relative path only when target is inside root.
func relativeInside(root, target string) (string, error) {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%s is outside %s", target, root)
	}
	return rel, nil
}
