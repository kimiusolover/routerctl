package releaseverify

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestVerifyAcceptsMatchingMetadata(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "manifest.json", `{"artifacts":[{"name":"routerctl-linux-amd64","sha256":"`+digest+`"}]}`)
	write(t, dir, "SHA256SUMS", digest+"  routerctl-linux-amd64\n")
	write(t, dir, "provenance.json", `{"subject":[{"name":"routerctl-linux-amd64","digest":{"sha256":"`+digest+`"}}]}`)
	if err := Verify(filepath.Join(dir, "manifest.json"), filepath.Join(dir, "SHA256SUMS"), filepath.Join(dir, "provenance.json")); err != nil {
		t.Fatal(err)
	}
}
func TestVerifyAcceptsDSSEProvenance(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "manifest.json", `{"artifacts":[{"name":"a","sha256":"`+digest+`"}]}`)
	write(t, dir, "SHA256SUMS", digest+"  a\n")
	payload := base64.StdEncoding.EncodeToString([]byte(`{"subject":[{"name":"a","digest":{"sha256":"` + digest + `"}}]}`))
	write(t, dir, "provenance.json", `{"payload":"`+payload+`"}`)
	if err := Verify(filepath.Join(dir, "manifest.json"), filepath.Join(dir, "SHA256SUMS"), filepath.Join(dir, "provenance.json")); err != nil {
		t.Fatal(err)
	}
}
func TestVerifyReportsMissingAndMismatchedArtifacts(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "manifest.json", `{"artifacts":[{"name":"a","sha256":"`+digest+`"}]}`)
	write(t, dir, "SHA256SUMS", strings.Repeat("b", 64)+"  a\n")
	write(t, dir, "provenance.json", `{"subject":[{"name":"b","digest":{"sha256":"`+digest+`"}}]}`)
	err := Verify(filepath.Join(dir, "manifest.json"), filepath.Join(dir, "SHA256SUMS"), filepath.Join(dir, "provenance.json"))
	if err == nil || !strings.Contains(err.Error(), "a is missing from provenance.json") || !strings.Contains(err.Error(), "b is missing from manifest, SHA256SUMS") {
		t.Fatalf("error = %v", err)
	}
}

func TestFailureFixtures(t *testing.T) {
	fixtures := []struct {
		name string
		want string
	}{
		{name: "sha-mismatch", want: "a digest mismatch"},
		{name: "asset-missing", want: "a is missing from provenance.json"},
		{name: "duplicate-artifact", want: `manifest: duplicate artifact "a"`},
		{name: "invalid-dsse", want: "parse provenance.json DSSE payload"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			dir := filepath.Join("testdata", "failure", fixture.name)
			err := Verify(
				filepath.Join(dir, "manifest.json"),
				filepath.Join(dir, "SHA256SUMS"),
				filepath.Join(dir, "provenance.json"),
			)
			if err == nil || !strings.Contains(err.Error(), fixture.want) {
				t.Fatalf("Verify() error = %v, want substring %q", err, fixture.want)
			}
		})
	}
}
func write(t *testing.T, dir, name, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
