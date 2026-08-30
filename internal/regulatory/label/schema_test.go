package label_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

func findSchemaFile(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "schemas", "regulatory-label-bundle.schema.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find schemas/regulatory-label-bundle.schema.json")
	return ""
}

func toGenericJSON(t *testing.T, v any) any {
	t.Helper()
	yamlBytes, err := yaml.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(yamlBytes, &raw); err != nil {
		t.Fatal(err)
	}
	jsonBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(jsonBytes, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRegulatoryLabelBundle_JSONSchema(t *testing.T) {
	schemaPath := findSchemaFile(t)
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		t.Fatalf("failed to compile JSON schema %s: %v", schemaPath, err)
	}

	t.Run("ValidBundleConformsToSchema", func(t *testing.T) {
		bundle := validBundleFixture()
		val := toGenericJSON(t, &bundle)
		if err := schema.Validate(val); err != nil {
			t.Fatalf("valid bundle failed schema validation: %v", err)
		}
	})

	t.Run("RoleToFileMismatchRejection", func(t *testing.T) {
		bundle := validBundleFixture()
		// Mismatch role and file: device_identity_crop with giteki_mark_and_number_crop.png
		bundle.Artifacts[0].File = "giteki_mark_and_number_crop.png"
		val := toGenericJSON(t, &bundle)
		if err := schema.Validate(val); err == nil {
			t.Error("expected schema validation failure for mismatched role/file, got nil")
		}
	})

	t.Run("MissingMandatoryRoleRejection", func(t *testing.T) {
		bundle := validBundleFixture()
		// Only 1 artifact
		bundle.Artifacts = bundle.Artifacts[:1]
		val := toGenericJSON(t, &bundle)
		if err := schema.Validate(val); err == nil {
			t.Error("expected schema validation failure for missing mandatory role, got nil")
		}
	})

	t.Run("InvalidVersionRejection", func(t *testing.T) {
		bundle := validBundleFixture()
		bundle.Version = "2.0"
		val := toGenericJSON(t, &bundle)
		if err := schema.Validate(val); err == nil {
			t.Error("expected schema validation failure for invalid version, got nil")
		}
	})

	t.Run("InvalidSHA256Rejection", func(t *testing.T) {
		bundle := validBundleFixture()
		bundle.Source.SHA256 = "short-hash"
		val := toGenericJSON(t, &bundle)
		if err := schema.Validate(val); err == nil {
			t.Error("expected schema validation failure for invalid SHA-256, got nil")
		}
	})

	t.Run("InvalidStatusRejection", func(t *testing.T) {
		bundle := validBundleFixture()
		bundle.Observations.Reviewed.Status = "invalid_status"
		val := toGenericJSON(t, &bundle)
		if err := schema.Validate(val); err == nil {
			t.Error("expected schema validation failure for invalid status, got nil")
		}
	})

	t.Run("OCRSuccessWithoutCandidatesRejection", func(t *testing.T) {
		bundle := validBundleFixture()
		bundle.Observations.OCRCandidates.Status = "success"
		bundle.Observations.OCRCandidates.CertificationNumbers = nil
		val := toGenericJSON(t, &bundle)
		if err := schema.Validate(val); err == nil {
			t.Error("expected schema failure for OCR success without candidates, got nil")
		}
	})

	t.Run("OCRUnavailableWithoutErrorRejection", func(t *testing.T) {
		bundle := validBundleFixture()
		bundle.Observations.OCRCandidates.Status = "unavailable"
		bundle.Observations.OCRCandidates.CertificationNumbers = nil
		bundle.Observations.OCRCandidates.Error = ""
		val := toGenericJSON(t, &bundle)
		if err := schema.Validate(val); err == nil {
			t.Error("expected schema failure for OCR unavailable without error, got nil")
		}
	})

	t.Run("OCRNoMatchWithCandidatesRejection", func(t *testing.T) {
		bundle := validBundleFixture()
		bundle.Observations.OCRCandidates.Status = "no_match"
		bundle.Observations.OCRCandidates.CertificationNumbers = []string{"201-230283"}
		val := toGenericJSON(t, &bundle)
		if err := schema.Validate(val); err == nil {
			t.Error("expected schema failure for OCR no_match with candidates, got nil")
		}
	})
}
