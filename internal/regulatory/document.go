// Package regulatory imports local certification documents without querying
// authority search sites.
package regulatory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/example/routerctl/internal/regulatory/importer"
)

func ReadDocument(path string) (importer.Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return importer.Document{}, err
	}
	text := raw
	if filepath.Ext(path) == ".pdf" {
		text, err = exec.Command("pdftotext", "-layout", path, "-").Output()
		if err != nil {
			return importer.Document{}, fmt.Errorf("regulatory: extract PDF text: %w", err)
		}
	}
	sum := sha256.Sum256(raw)
	return importer.Document{Name: filepath.Base(path), SHA256: hex.EncodeToString(sum[:]), Text: string(text)}, nil
}
