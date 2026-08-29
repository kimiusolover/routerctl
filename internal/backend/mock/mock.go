package mock

import (
	"context"

	"github.com/example/routerctl/internal/backend"
)

type Backend struct{}

func (Backend) Name() string { return "mock" }

func (Backend) Resolve(_ context.Context, req backend.Request) (backend.Artifact, error) {
	return backend.Artifact{Path: "mock://" + req.Device + "/" + req.Target}, nil
}

func (Backend) Build(_ context.Context, req backend.BuildRequest) (backend.BuildResult, error) {
	return backend.BuildResult{Artifacts: []backend.Artifact{{
		Path:   "mock://build/" + req.Device + "/" + req.Profile,
		Digest: "sha256:ec864fe99b539704b8872ac591067ef22d836a8d942087f2dba274b301ebe6e5",
	}}}, nil
}
