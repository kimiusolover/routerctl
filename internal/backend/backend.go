package backend

import "context"

type Artifact struct {
	Path   string
	Digest string
}

type Request struct {
	Device string
	Target string
}

type Backend interface {
	Name() string
	Resolve(context.Context, Request) (Artifact, error)
}
