package backend

import (
	"context"
	"errors"
)

// ErrBuildUnsupported is returned by backends that can resolve an existing
// artifact but cannot produce one. For example, the GitHub backend is a
// release resolver rather than a build service.
var ErrBuildUnsupported = errors.New("backend: build is not supported")

type Artifact struct {
	Path   string
	Digest string
}

type Request struct {
	Device string
	Target string
}

// BuildRequest describes a host-side build without coupling the core to a
// firmware tree or a specific build tool. WorkspaceRoot and OutputDir are
// absolute paths owned by the caller; Profile selects the product build
// configuration inside that workspace.
type BuildRequest struct {
	Device        string
	Profile       string
	WorkspaceRoot string
	OutputDir     string
}

// BuildResult is the complete, immutable set of artifacts produced by one
// build invocation. Each artifact must carry a SHA-256 digest.
type BuildResult struct {
	Artifacts []Artifact
}

type Backend interface {
	Name() string
	Resolve(context.Context, Request) (Artifact, error)
	Build(context.Context, BuildRequest) (BuildResult, error)
}
