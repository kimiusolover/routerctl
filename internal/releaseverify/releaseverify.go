// Package releaseverify checks that all release metadata names the same files
// with the same SHA-256 digests.
package releaseverify

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

var sha256 = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

type artifact struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}
type releaseManifest struct {
	Artifacts []artifact `json:"artifacts"`
}
type statement struct {
	Subject []struct {
		Name   string            `json:"name"`
		Digest map[string]string `json:"digest"`
	} `json:"subject"`
}

// Verify reads a JSON release manifest, a sha256sum-format checksum file, and
// an in-toto Statement (or a DSSE envelope containing one). It reports every
// disagreement, including an artifact that appears in only some inputs.
func Verify(manifestPath, sumsPath, provenancePath string) error {
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return err
	}
	sums, err := readSums(sumsPath)
	if err != nil {
		return err
	}
	provenance, err := readProvenance(provenancePath)
	if err != nil {
		return err
	}
	names := map[string]struct{}{}
	for _, set := range []map[string]string{manifest, sums, provenance} {
		for name := range set {
			names[name] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	var problems []string
	for _, name := range ordered {
		m, mok := manifest[name]
		s, sok := sums[name]
		p, pok := provenance[name]
		if !mok || !sok || !pok {
			missing := make([]string, 0, 2)
			if !mok {
				missing = append(missing, "manifest")
			}
			if !sok {
				missing = append(missing, "SHA256SUMS")
			}
			if !pok {
				missing = append(missing, "provenance.json")
			}
			problems = append(problems, fmt.Sprintf("%s is missing from %s", name, strings.Join(missing, ", ")))
			continue
		}
		if m != s || m != p {
			problems = append(problems, fmt.Sprintf("%s digest mismatch (manifest=%s SHA256SUMS=%s provenance=%s)", name, m, s, p))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("release metadata verification failed:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func readManifest(file string) (map[string]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m releaseManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	result := make(map[string]string, len(m.Artifacts))
	for _, a := range m.Artifacts {
		if err := add(result, a.Name, a.SHA256); err != nil {
			return nil, fmt.Errorf("manifest: %w", err)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("manifest: no artifacts")
	}
	return result, nil
}

func readSums(file string) (map[string]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("read SHA256SUMS: %w", err)
	}
	defer f.Close()
	result := map[string]string{}
	s := bufio.NewScanner(f)
	line := 0
	for s.Scan() {
		line++
		fields := strings.Fields(s.Text())
		if len(fields) != 2 {
			return nil, fmt.Errorf("SHA256SUMS: line %d must contain a digest and filename", line)
		}
		if err := add(result, strings.TrimPrefix(fields[1], "*"), fields[0]); err != nil {
			return nil, fmt.Errorf("SHA256SUMS: line %d: %w", line, err)
		}
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("read SHA256SUMS: %w", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("SHA256SUMS: no artifacts")
	}
	return result, nil
}

func readProvenance(file string) (map[string]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read provenance.json: %w", err)
	}
	var envelope struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parse provenance.json: %w", err)
	}
	if envelope.Payload != "" {
		data, err = base64.StdEncoding.DecodeString(envelope.Payload)
		if err != nil {
			return nil, fmt.Errorf("parse provenance.json DSSE payload: %w", err)
		}
	}
	var p statement
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&p); err != nil {
		return nil, fmt.Errorf("parse provenance.json statement: %w", err)
	}
	result := make(map[string]string, len(p.Subject))
	for _, subject := range p.Subject {
		if err := add(result, subject.Name, subject.Digest["sha256"]); err != nil {
			return nil, fmt.Errorf("provenance.json: %w", err)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("provenance.json: no subjects")
	}
	return result, nil
}

func add(set map[string]string, name, digest string) error {
	if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") {
		return fmt.Errorf("invalid artifact name %q", name)
	}
	if !sha256.MatchString(digest) {
		return fmt.Errorf("invalid SHA-256 for %q", name)
	}
	if _, exists := set[name]; exists {
		return fmt.Errorf("duplicate artifact %q", name)
	}
	set[name] = strings.ToLower(digest)
	return nil
}
