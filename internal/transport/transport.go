package transport

import "context"

type Target struct {
	Address string
}

type Transport interface {
	Name() string
	Probe(context.Context, Target) error
	Deploy(context.Context, Target, string) error
}
