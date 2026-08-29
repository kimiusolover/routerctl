package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/example/routerctl/internal/artifactverify"
	"github.com/example/routerctl/internal/backend"
	githubbackend "github.com/example/routerctl/internal/backend/github"
	nativebackend "github.com/example/routerctl/internal/backend/native"
	"github.com/example/routerctl/internal/firmwaregate"
	"github.com/example/routerctl/internal/manifest"
	"github.com/example/routerctl/internal/planner"
	"github.com/example/routerctl/internal/regulatory"
	"github.com/example/routerctl/internal/regulatory/derive"
	"github.com/example/routerctl/internal/regulatory/explain"
	"github.com/example/routerctl/internal/regulatory/importer/mic"
	"github.com/example/routerctl/internal/regulatory/label"
	"github.com/example/routerctl/internal/regulatory/profile"
	"github.com/example/routerctl/internal/regulatory/validate"
	"github.com/example/routerctl/internal/releaseverify"
	verifycmd "github.com/example/routerctl/internal/verify"
	"gopkg.in/yaml.v3"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func Run(args []string, info BuildInfo) error {
	if len(args) == 0 {
		usage(os.Stdout)
		return nil
	}

	switch args[0] {
	case "build":
		if len(args) != 2 {
			return errors.New("usage: routerctl build <manifest>")
		}
		return runBuild(args[1])
	case "regulatory":
		return runRegulatory(args[1:])
	case "version":
		fmt.Printf("routerctl %s (commit=%s date=%s)\n", info.Version, info.Commit, info.Date)
		return nil
	case "inspect":
		if len(args) != 2 {
			return errors.New("usage: routerctl inspect <manifest>")
		}
		m, err := manifest.Load(args[1])
		if err != nil {
			return err
		}
		return printJSON(os.Stdout, m)
	case "plan":
		if len(args) != 2 {
			return errors.New("usage: routerctl plan <manifest>")
		}
		m, err := manifest.Load(args[1])
		if err != nil {
			return err
		}
		p, err := planner.Create(m)
		if err != nil {
			return err
		}
		return printJSON(os.Stdout, p)
	case "verify":
		if len(args) != 2 {
			return errors.New("usage: routerctl verify <manifest>")
		}
		m, err := manifest.Load(args[1])
		if err != nil {
			return err
		}
		result := verifycmd.Manifest(m)
		if !result.OK {
			return fmt.Errorf("verification failed: %v", result.Errors)
		}
		fmt.Println("OK")
		return nil
	case "resolve":
		if len(args) != 2 {
			return errors.New("usage: routerctl resolve <manifest>")
		}
		m, err := manifest.Load(args[1])
		if err != nil {
			return err
		}
		result := verifycmd.Manifest(m)
		if !result.OK {
			return fmt.Errorf("verification failed: %v", result.Errors)
		}
		if m.Spec.Backend != "github" {
			return fmt.Errorf("resolve: backend %q is not supported", m.Spec.Backend)
		}
		resolver, err := githubbackend.New(githubbackend.Config{
			Repository: m.Spec.GitHub.Repository,
			Tag:        m.Spec.GitHub.Tag,
			Token:      os.Getenv("GITHUB_TOKEN"),
		})
		if err != nil {
			return err
		}
		artifact, err := resolver.Resolve(context.Background(), backend.Request{
			Device: m.Spec.Device,
			Target: m.Spec.Target,
		})
		if err != nil {
			return err
		}
		return printJSON(os.Stdout, artifact)
	case "verify-release":
		if len(args) != 4 {
			return errors.New("usage: routerctl verify-release <manifest.json> <SHA256SUMS> <provenance.json>")
		}
		if err := releaseverify.Verify(args[1], args[2], args[3]); err != nil {
			return err
		}
		fmt.Println("OK")
		return nil
	case "help", "-h", "--help":
		usage(os.Stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runBuild(manifestPath string) error {
	m, err := manifest.Load(manifestPath)
	if err != nil {
		return err
	}
	result := verifycmd.Manifest(m)
	if !result.OK {
		return fmt.Errorf("verification failed: %v", result.Errors)
	}
	if m.Spec.Backend != "native" {
		return fmt.Errorf("build: backend %q is not supported", m.Spec.Backend)
	}
	manifestDir, err := filepath.Abs(filepath.Dir(manifestPath))
	if err != nil {
		return fmt.Errorf("build: resolve manifest directory: %w", err)
	}
	repository, err := resolveInside(manifestDir, m.Spec.Build.Repository)
	if err != nil {
		return fmt.Errorf("build: repository: %w", err)
	}
	info, err := os.Stat(repository)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("build: repository is not a directory: %s", repository)
	}
	// This is intentionally before command execution: router-firmware is the
	// authority on whether it is safe to assemble an image for this device.
	if err := firmwaregate.Check(repository, m.Spec.Device); err != nil {
		return err
	}
	output, err := resolveInside(repository, m.Spec.Build.Output)
	if err != nil {
		return fmt.Errorf("build: output: %w", err)
	}
	b, err := nativebackend.New(nativebackend.Config{Command: m.Spec.Build.Command})
	if err != nil {
		return err
	}
	built, err := b.Build(context.Background(), backend.BuildRequest{
		Device: m.Spec.Device, Profile: m.Spec.Build.Profile, WorkspaceRoot: repository, OutputDir: filepath.Dir(output),
	})
	if err != nil {
		return err
	}
	for _, artifact := range built.Artifacts {
		if filepath.Clean(artifact.Path) == output {
			if err := artifactverify.Verify(output, artifact, m.Spec.Artifact.Expected); err != nil {
				return err
			}
			return printJSON(os.Stdout, artifact)
		}
	}
	return fmt.Errorf("build: expected output was not produced: %s", output)
}

func resolveInside(root, path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return "", errors.New("must be a non-empty relative path")
	}
	full := filepath.Clean(filepath.Join(root, path))
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("must remain inside its root")
	}
	return full, nil
}

type cropList []label.CropTarget

func (c *cropList) String() string {
	var parts []string
	for _, target := range *c {
		parts = append(parts, fmt.Sprintf("%s=%s", target.Role, target.Geometry))
	}
	return strings.Join(parts, ", ")
}

func (c *cropList) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid crop format %q (must be role=WxH+X+Y)", value)
	}
	*c = append(*c, label.CropTarget{
		Role:     strings.TrimSpace(parts[0]),
		Geometry: strings.TrimSpace(parts[1]),
	})
	return nil
}

func runRegulatoryLabelExtract(args []string) error {
	fs := flag.NewFlagSet("label extract", flag.ContinueOnError)
	var (
		imagePath   string
		outDir      string
		layoutID    string
		layoutPath  string
		srcSHA256   string
		vendor      string
		model       string
		revision    string
		certNumber  string
		status      string
		minConf     float64
		ocrPattern  string
		ocrLang     string
		ocrOptional bool
		noOCR       = true
		crops       cropList
	)
	fs.StringVar(&imagePath, "image", "", "Path to source label image")
	fs.StringVar(&outDir, "out", "", "Output bundle directory")
	fs.StringVar(&outDir, "output", "", "Output bundle directory (alias)")
	fs.StringVar(&layoutID, "layout-id", "", "Layout identifier")
	fs.StringVar(&layoutPath, "layout", "", "Reviewed YAML label-layout definition")
	fs.StringVar(&srcSHA256, "source-sha256", "", "SHA-256 hash of source image")
	fs.StringVar(&vendor, "vendor", "", "Vendor name")
	fs.StringVar(&model, "model", "", "Model name")
	fs.StringVar(&revision, "revision", "", "Hardware revision")
	fs.StringVar(&certNumber, "cert-number", "", "Reviewed certification number")
	fs.StringVar(&status, "status", "", "Review status")
	fs.Float64Var(&minConf, "min-conf", 60.0, "Minimum confidence score for OCR")
	fs.StringVar(&ocrPattern, "ocr-pattern", `\d{3}-\d{6}`, "Regex pattern for certification numbers")
	fs.StringVar(&ocrLang, "ocr-lang", "eng", "OCR language (default: eng)")
	fs.BoolVar(&ocrOptional, "ocr-optional", false, "Do not fail if OCR candidate extraction fails")
	fs.BoolVar(&noOCR, "no-ocr", true, "Disable OCR candidate extraction (default)")
	fs.Var(&crops, "crop", "Crop specification in format role=WxH+X+Y")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if imagePath == "" {
		return errors.New("label extract: --image is required")
	}
	if outDir == "" {
		return errors.New("label extract: --out is required")
	}
	if layoutPath != "" {
		if layoutID != "" || len(crops) != 0 {
			return errors.New("label extract: --layout cannot be combined with --layout-id or --crop")
		}
		loaded, loadedCrops, err := label.LoadLayout(layoutPath)
		if err != nil {
			return err
		}
		layoutID, crops = loaded.Metadata.Name, cropList(loadedCrops)
	}
	if layoutID == "" || len(crops) == 0 {
		return errors.New("label extract: --layout is required")
	}

	var pattern *regexp.Regexp
	if !noOCR && ocrPattern != "" {
		var err error
		pattern, err = regexp.Compile(ocrPattern)
		if err != nil {
			return fmt.Errorf("invalid ocr-pattern regex: %w", err)
		}
	}

	bundle, err := label.ExtractAndCommitBundleWithOptions(context.Background(), label.ExtractOptions{
		SourcePath:   imagePath,
		SourceSHA256: srcSHA256,
		LayoutID:     layoutID,
		FinalDir:     outDir,
		Crops:        crops,
		RunOCR:       !noOCR,
		OCROptional:  ocrOptional,
		OCRLang:      ocrLang,
		OCRPattern:   pattern,
		MinConf:      minConf,
		Reviewed: label.ReviewedSpec{
			Vendor:              vendor,
			Model:               model,
			HardwareRevision:    revision,
			CertificationNumber: certNumber,
			Status:              status,
		},
	})
	if err != nil {
		return err
	}

	yamlData, err := yaml.Marshal(bundle)
	if err != nil {
		return err
	}
	fmt.Print(string(yamlData))
	return nil
}

func runRegulatory(args []string) error {
	if len(args) >= 2 && args[0] == "label" && args[1] == "extract" {
		return runRegulatoryLabelExtract(args[2:])
	}
	if len(args) == 3 && args[0] == "label" && args[1] == "verify" {
		_, err := label.VerifyBundleDirectory(args[2])
		if err != nil {
			return err
		}
		fmt.Println("OK")
		return nil
	}
	if len(args) == 18 && args[0] == "bundle" && args[1] == "create" {
		values := map[string]string{}
		for i := 2; i < len(args); i += 2 {
			values[args[i]] = args[i+1]
		}
		bundle, err := regulatory.CreateBundle(values["--authority"], values["--number"], values["--source-url"], values["--number-evidence"], values["--report"], values["--vendor"], values["--model"], values["--revision"])
		if err != nil {
			return err
		}
		return printJSON(os.Stdout, bundle)
	}
	if len(args) == 4 && args[0] == "import" && args[1] == "mic" && args[2] == "--bundle" {
		bundle, err := regulatory.ReadBundle(args[3])
		if err != nil {
			return err
		}
		if bundle.Authority != "MIC" || bundle.Jurisdiction != "JP" {
			return errors.New("mic: bundle must be for JP/MIC")
		}
		for _, entry := range bundle.Documents {
			if entry.Role != "test_report" {
				continue
			}
			doc, err := regulatory.ReadDocument(filepath.Join(filepath.Dir(args[3]), entry.File))
			if err != nil {
				return err
			}
			record, err := (mic.Importer{}).ExtractWithNumber(doc, bundle.CertificationNumber, bundle.NumberSource)
			if err != nil {
				return err
			}
			if err := mic.MatchesDevice(record, bundle.Device); err != nil {
				return err
			}
			return printJSON(os.Stdout, record)
		}
		return errors.New("mic: bundle has no test_report document")
	}
	if len(args) == 3 && args[0] == "import" && args[1] == "mic" {
		doc, err := regulatory.ReadDocument(args[2])
		if err != nil {
			return err
		}
		adapter := mic.Importer{}
		if !adapter.Detect(doc) {
			return errors.New("mic: document does not look like an MIC certification document")
		}
		record, err := adapter.Extract(doc)
		if err != nil {
			return err
		}
		return printJSON(os.Stdout, record)
	}
	if len(args) == 2 && args[0] == "validate" {
		record, err := regulatory.ReadRecord(args[1])
		if err != nil {
			return err
		}
		if err := validate.Record(record); err != nil {
			return err
		}
		fmt.Println("OK")
		return nil
	}
	if len(args) == 3 && args[0] == "profile" && args[1] == "check" {
		p, err := profile.Read(args[2])
		if err != nil {
			return err
		}
		if err := profile.Validate(p); err != nil {
			return err
		}
		if !p.TXPermitted() {
			fmt.Println("VALID: TX DENIED (evidence is not verified)")
			return nil
		}
		fmt.Println("VALID: TX may be evaluated against runtime hardware/driver constraints")
		return nil
	}
	if len(args) == 2 && args[0] == "derive" {
		record, err := regulatory.ReadRecord(args[1])
		if err != nil {
			return err
		}
		derived, err := derive.Constraints(record)
		if err != nil {
			return err
		}
		return printJSON(os.Stdout, derived)
	}
	if len(args) == 3 && args[0] == "explain" {
		derived, err := regulatory.ReadDerived(args[1])
		if err != nil {
			return err
		}
		text, err := explain.Constraint(derived, args[2])
		if err != nil {
			return err
		}
		fmt.Print(text)
		return nil
	}
	return errors.New("usage: routerctl regulatory import mic <document.pdf|document.txt> | validate <record.json> | derive <record.json> | explain <derived.json> <key> | profile check <profile.yaml> | label extract [flags] | label verify <bundle-dir>")
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `routerctl - router OS host-side orchestrator

Usage:
  routerctl version
  routerctl inspect <manifest>
  routerctl plan <manifest>
  routerctl verify <manifest>
  routerctl resolve <manifest>
  routerctl verify-release <manifest.json> <SHA256SUMS> <provenance.json>
  routerctl regulatory import mic <document.pdf|document.txt>
  routerctl regulatory import mic --bundle <bundle.json>
  routerctl regulatory bundle create --authority MIC --number <number> --source-url <url> --number-evidence <file> --report <file> --vendor <vendor> --model <model> --revision <revision>
  routerctl regulatory validate <record.json>
  routerctl regulatory derive <record.json>
  routerctl regulatory explain <derived.json> <key>
  routerctl regulatory profile check <profile.yaml>
  routerctl regulatory label extract --image <image> --layout <layout.yaml> --out <dir>
  routerctl regulatory label verify <bundle-dir>`)
}
