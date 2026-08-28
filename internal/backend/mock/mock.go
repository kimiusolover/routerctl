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
