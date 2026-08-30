package regulatory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBundleAcceptsYAMLFixture(t *testing.T) {
	file := filepath.Join("..", "..", "examples", "ax23v", "regulatory", "JP", "evidence-bundle.yaml")
	bundle, err := ReadBundle(file)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.CertificationNumber != "201-230283" || len(bundle.Documents) != 1 {
		t.Fatalf("bundle = %#v", bundle)
	}
}

func TestReadBundleRejectsAlteredEvidence(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bundle.json")
	data := `{"apiVersion":"routerctl.regulatory.evidence-bundle/v1","jurisdiction":"JP","authority":"MIC","certificationNumber":"201-x","numberSource":{"evidenceFile":"missing","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"documents":[{"role":"test_report","file":"missing","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`
	if err := os.WriteFile(bad, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ReadBundle(bad)
	if err == nil || !strings.Contains(err.Error(), "missing required evidence") {
		t.Fatalf("error = %v", err)
	}
}

func TestReadBundleRejectsOfficialSearchWithoutManualCheckMetadata(t *testing.T) {
	dir := t.TempDir()
	fixtureDir := filepath.Join("..", "..", "examples", "ax23v", "regulatory", "JP")
	for _, name := range []string{"number-source-201-230283.txt", "mic-report.txt"} {
		data, err := os.ReadFile(filepath.Join(fixtureDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bundle, err := os.ReadFile(filepath.Join(fixtureDir, "evidence-bundle.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	bundle = []byte(strings.Replace(string(bundle), "  checkedBy: synthetic-fixture\n", "", 1))
	path := filepath.Join(dir, "evidence-bundle.yaml")
	if err := os.WriteFile(path, bundle, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ReadBundle(path)
	if err == nil || !strings.Contains(err.Error(), "checkedBy") {
		t.Fatalf("error = %v", err)
	}
}
