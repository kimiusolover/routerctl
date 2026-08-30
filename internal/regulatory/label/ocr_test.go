package label_test

import (
	"context"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/example/routerctl/internal/regulatory/label"
)

//go:embed testdata/synthetic_label.png
var syntheticLabelPNG []byte

//go:embed testdata/eng.traineddata
var engTrainedData []byte

//go:embed testdata/configs/tsv
var tsvConfigFile []byte

func setupTessdata(t *testing.T, tempDir string) string {
	t.Helper()
	tessdataDir := filepath.Join(tempDir, "tessdata")
	if err := os.MkdirAll(filepath.Join(tessdataDir, "configs"), 0755); err != nil {
		t.Fatalf("failed to create tessdata dir: %v", err)
	}
	if len(engTrainedData) > 0 {
		if err := os.WriteFile(filepath.Join(tessdataDir, "eng.traineddata"), engTrainedData, 0644); err != nil {
			t.Fatalf("failed to write eng.traineddata: %v", err)
		}
	}
	if len(tsvConfigFile) > 0 {
		if err := os.WriteFile(filepath.Join(tessdataDir, "configs", "tsv"), tsvConfigFile, 0644); err != nil {
			t.Fatalf("failed to write tsv config: %v", err)
		}
	}
	t.Setenv("TESSDATA_PREFIX", tessdataDir)
	return tessdataDir
}

func TestParseTSVAndExtractCandidates_UnitTest(t *testing.T) {
	tsvData := "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
		"5\t1\t1\t1\t1\t1\t10\t20\t30\t10\t95.0\t201-\n" +
		"5\t1\t1\t1\t1\t2\t45\t20\t40\t10\t90.0\t230283\n"
	pattern := regexp.MustCompile(`\d{3}-\d{6}`)
	candidates, err := label.ParseTSVAndExtractCandidates(strings.NewReader(tsvData), pattern, 60.0)
	if err != nil {
		t.Fatalf("ParseTSVAndExtractCandidates failed: %v", err)
	}

	if len(candidates) != 1 || candidates[0] != "201-230283" {
		t.Errorf("expected ['201-230283'], got %v", candidates)
	}
}

func TestExtractCertificationNumbers_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract CLI tool not found, skipping E2E test")
	}

	// Prepare temp test environment with embedded fixture and self-contained tessdata
	tempDir := t.TempDir()
	imgPath := filepath.Join(tempDir, "label_fixture.png")
	if err := os.WriteFile(imgPath, syntheticLabelPNG, 0644); err != nil {
		t.Fatalf("failed to write embedded synthetic fixture image: %v", err)
	}

	setupTessdata(t, tempDir)

	pattern := regexp.MustCompile(`\d{3}-\d{6}`)
	candidates, err := label.ExtractCertificationNumbers(context.Background(), imgPath, pattern, 60.0)
	if err != nil {
		t.Fatalf("ExtractCertificationNumbers failed: %v", err)
	}

	if len(candidates) != 1 || candidates[0] != "201-230283" {
		t.Errorf("expected candidates to be ['201-230283'], got %v", candidates)
	}
}
