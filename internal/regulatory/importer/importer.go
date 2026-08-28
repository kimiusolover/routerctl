// Package importer defines country-specific certification document adapters.
package importer

import "github.com/example/routerctl/internal/regulatory/model"

type Document struct {
	Name   string
	SHA256 string
	Text   string
}

type Importer interface {
	Detect(Document) bool
	Extract(Document) (*model.CertificationRecord, error)
}
