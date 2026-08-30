package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/example/routerctl/internal/artifactverify"
	"github.com/example/routerctl/internal/backend"
	githubbackend "github.com/example/routerctl/internal/backend/github"
	nativebackend "github.com/example/routerctl/internal/backend/native"
	"github.com/example/routerctl/internal/firmwaregate"
	"github.com/example/routerctl/internal/gitops"
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
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func Run(args []string, info BuildInfo) error {
	if len(args) > 0 && (args[0] == "interactive" || args[0] == "--interactive" || args[0] == "-i") {
		return runInteractive(os.Stdin, os.Stdout, info)
	}
	if len(args) == 0 {
		if isTerminal(os.Stdin) {
			return runInteractive(os.Stdin, os.Stdout, info)
		}
		nextSteps(os.Stdout)
		return nil
	}

	switch args[0] {
	case "git":
		return runGit(args[1:])
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

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// runInteractive is a REPL: commands may be entered repeatedly, as in parted.
// Commands with omitted required parameters ask only for those parameters.
func runInteractive(in io.Reader, out io.Writer, info BuildInfo) error {
	if input, ok := in.(*os.File); ok && term.IsTerminal(int(input.Fd())) {
		return runInteractiveTerminal(input, out, info)
	}
	reader := bufio.NewReader(in)
	fmt.Fprintln(out, "routerctl interactive mode")
	fmt.Fprintln(out, "Enter `help` for commands, or `quit` to exit.")
	for {
		fmt.Fprint(out, "[routerctl]# ")
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" && errors.Is(err, io.EOF) {
			return nil
		}
		if line == "" {
			continue
		}
		quit, commandErr := executeInteractiveLine(reader, out, line, info)
		if commandErr != nil {
			return commandErr
		}
		if quit {
			return nil
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
}

func executeInteractiveLine(reader *bufio.Reader, out io.Writer, line string, info BuildInfo) (bool, error) {
	args, err := splitCommandLine(line)
	if err != nil {
		fmt.Fprintf(out, "routerctl: %v\n", err)
		return false, nil
	}
	switch args[0] {
	case "quit", "exit":
		return true, nil
	case "help", "?":
		interactiveUsage(out)
		return false, nil
	}
	args, err = fillInteractiveArgs(reader, out, args)
	if err != nil {
		fmt.Fprintf(out, "routerctl: %v\n", err)
		return false, nil
	}
	if len(args) >= 2 && args[0] == "git" && (args[1] == "commit" || args[1] == "sync") {
		answer, confirmErr := promptDefault(reader, out, "Proceed with Git changes? [y/N]", "n")
		if confirmErr != nil {
			return false, confirmErr
		}
		if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
			return false, nil
		}
	}
	if commandErr := Run(args, info); commandErr != nil {
		fmt.Fprintf(out, "routerctl: %v\n", commandErr)
	}
	return false, nil
}

func runInteractiveTerminal(in *os.File, out io.Writer, info BuildInfo) error {
	reader := bufio.NewReader(in)
	fmt.Fprintln(out, "routerctl interactive mode")
	fmt.Fprintln(out, "Enter `help` for commands, Tab to complete, or `quit` / Ctrl+D to exit.")
	for {
		state, err := term.MakeRaw(int(in.Fd()))
		if err != nil {
			return err
		}
		line, readErr := readInteractiveLine(reader, out)
		if restoreErr := term.Restore(int(in.Fd()), state); restoreErr != nil {
			return restoreErr
		}
		fmt.Fprintln(out)
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
		quit, err := executeInteractiveLine(reader, out, line, info)
		if err != nil {
			return err
		}
		if quit {
			return nil
		}
	}
}

func readInteractiveLine(reader *bufio.Reader, out io.Writer) (string, error) {
	buffer := []rune{}
	fmt.Fprint(out, "[routerctl]# ")
	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			return "", err
		}
		switch r {
		case '\r', '\n':
			return string(buffer), nil
		case 3:
			return "", errors.New("interrupted")
		case 4:
			if len(buffer) == 0 {
				return "", io.EOF
			}
		case 8, 127:
			if len(buffer) > 0 {
				buffer = buffer[:len(buffer)-1]
				fmt.Fprint(out, "\b \b")
			}
		case '\t':
			buffer = completeInteractiveLine(out, buffer)
		default:
			if r >= 32 {
				buffer = append(buffer, r)
				fmt.Fprintf(out, "%c", r)
			}
		}
	}
}

func completeInteractiveLine(out io.Writer, buffer []rune) []rune {
	line := string(buffer)
	prefix := line[strings.LastIndexAny(line, " \t")+1:]
	candidates := completionCandidates(strings.Fields(line), strings.HasSuffix(line, " "))
	matches := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, prefix) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		fmt.Fprint(out, "\a")
		return buffer
	}
	if len(matches) == 1 {
		line = line[:len(line)-len(prefix)] + matches[0] + " "
		fmt.Fprint(out, matches[0][len(prefix):]+" ")
		return []rune(line)
	}
	common := longestCommonPrefix(matches)
	if common != prefix {
		line = line[:len(line)-len(prefix)] + common
		fmt.Fprint(out, common[len(prefix):])
		return []rune(line)
	}
	fmt.Fprintf(out, "\r\n%s\r\n[routerctl]# %s", strings.Join(matches, "  "), line)
	return buffer
}

func completionCandidates(words []string, trailingSpace bool) []string {
	if len(words) == 0 {
		return []string{"build", "git", "help", "inspect", "plan", "quit", "regulatory", "resolve", "verify", "verify-release", "version"}
	}
	if len(words) == 1 && trailingSpace && isManifestCommand(words[0]) {
		root, err := os.Getwd()
		if err != nil {
			return nil
		}
		candidates := findDeviceManifests(root)
		paths := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			paths = append(paths, candidate.Path)
		}
		return paths
	}
	if len(words) == 1 && !trailingSpace {
		return completionCandidates(nil, false)
	}
	if words[0] == "git" && len(words) <= 2 {
		return []string{"commit", "status", "sync"}
	}
	if words[0] != "regulatory" {
		return nil
	}
	if len(words) == 1 || (len(words) == 2 && !trailingSpace) {
		return []string{"bundle", "derive", "explain", "import", "label", "profile", "validate"}
	}
	switch words[1] {
	case "import":
		return []string{"mic"}
	case "profile":
		return []string{"check"}
	case "label":
		return []string{"extract", "verify"}
	case "bundle":
		return []string{"create"}
	}
	return nil
}

func isManifestCommand(command string) bool {
	return command == "build" || command == "inspect" || command == "plan" || command == "resolve" || command == "verify"
}

func longestCommonPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}
	prefix := values[0]
	for _, value := range values[1:] {
		for !strings.HasPrefix(value, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func interactiveUsage(out io.Writer) {
	fprintln := func(s string) { fmt.Fprintln(out, s) }
	fprintln("Commands are the same as the normal CLI. Examples:")
	fprintln("  inspect [manifest]        plan [manifest]        verify [manifest]")
	fprintln("  resolve [manifest]        build [manifest]")
	fprintln("  verify-release [manifest.json] [SHA256SUMS] [provenance.json]")
	fprintln("  git [status|commit|sync] [--repo PATH] [--message TEXT] [--dry-run]")
	fprintln("  regulatory import mic [document] | regulatory validate [record]")
	fprintln("  regulatory derive [record] | regulatory explain [derived] [key]")
	fprintln("  regulatory profile check [profile] | regulatory label extract [flags]")
	fprintln("  regulatory label verify [bundle-dir] | regulatory bundle create [flags]")
	fprintln("  version                    quit")
}

func fillInteractiveArgs(reader *bufio.Reader, out io.Writer, args []string) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}
	switch args[0] {
	case "inspect", "plan", "verify", "resolve", "build":
		if len(args) == 1 {
			value, err := promptRequired(reader, out, "Manifest path")
			return append(args, value), err
		}
	case "verify-release":
		for _, label := range []string{"Manifest JSON path", "SHA256SUMS path", "Provenance JSON path"} {
			if len(args) == 4 {
				break
			}
			value, err := promptRequired(reader, out, label)
			if err != nil {
				return nil, err
			}
			args = append(args, value)
		}
	case "git":
		if len(args) == 1 {
			value, err := promptRequired(reader, out, "Git command (status, commit, sync)")
			return append(args, value), err
		}
	case "regulatory":
		return fillRegulatoryArgs(reader, out, args)
	}
	return args, nil
}

func fillRegulatoryArgs(reader *bufio.Reader, out io.Writer, args []string) ([]string, error) {
	if len(args) == 1 {
		return args, nil
	}
	if len(args) == 3 && args[1] == "import" && args[2] == "mic" {
		value, err := promptRequired(reader, out, "MIC document path (or use --bundle PATH)")
		return append(args, value), err
	}
	if len(args) == 2 && (args[1] == "validate" || args[1] == "derive") {
		value, err := promptRequired(reader, out, "Certification record path")
		return append(args, value), err
	}
	if len(args) == 2 && args[1] == "explain" {
		derived, err := promptRequired(reader, out, "Derived constraints path")
		if err != nil {
			return nil, err
		}
		key, err := promptRequired(reader, out, "Constraint key")
		return append(args, derived, key), err
	}
	if len(args) == 3 && args[1] == "profile" && args[2] == "check" {
		value, err := promptRequired(reader, out, "Certification profile path")
		return append(args, value), err
	}
	if len(args) == 3 && args[1] == "label" && args[2] == "extract" {
		return interactiveLabelExtractArgs(reader, out)
	}
	if len(args) == 3 && args[1] == "label" && args[2] == "verify" {
		value, err := promptRequired(reader, out, "Label evidence bundle directory")
		return append(args, value), err
	}
	if len(args) == 3 && args[1] == "bundle" && args[2] == "create" {
		return interactiveBundleArgs(reader, out)
	}
	return args, nil
}

func splitCommandLine(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		return nil, errors.New("unfinished escape")
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	if len(args) == 0 {
		return nil, errors.New("empty command")
	}
	return args, nil
}

func interactiveLabelExtractArgs(reader *bufio.Reader, out io.Writer) ([]string, error) {
	image, err := promptRequired(reader, out, "Source label image path")
	if err != nil {
		return nil, err
	}
	layout, err := promptRequired(reader, out, "Reviewed label layout path")
	if err != nil {
		return nil, err
	}
	outDir, err := promptRequired(reader, out, "Output bundle directory")
	if err != nil {
		return nil, err
	}
	args := []string{"regulatory", "label", "extract", "--image", image, "--layout", layout, "--out", outDir}
	for _, field := range []struct{ label, flag string }{{"Vendor (optional)", "--vendor"}, {"Model (optional)", "--model"}, {"Hardware revision (optional)", "--revision"}, {"Certification number (optional)", "--cert-number"}, {"Review status (optional)", "--status"}} {
		value, err := promptDefault(reader, out, field.label, "")
		if err != nil {
			return nil, err
		}
		if value != "" {
			args = append(args, field.flag, value)
		}
	}
	return args, nil
}

func interactiveBundleArgs(reader *bufio.Reader, out io.Writer) ([]string, error) {
	args := []string{"regulatory", "bundle", "create"}
	for _, field := range []struct{ label, flag string }{{"Authority", "--authority"}, {"Certification number", "--number"}, {"Exact source URL", "--source-url"}, {"Checked at (RFC3339)", "--checked-at"}, {"Checked by", "--checked-by"}, {"Number evidence file", "--number-evidence"}, {"Test report file", "--report"}, {"Vendor", "--vendor"}, {"Model", "--model"}, {"Revision", "--revision"}} {
		value, err := promptRequired(reader, out, field.label)
		if err != nil {
			return nil, err
		}
		args = append(args, field.flag, value)
	}
	return args, nil
}

func promptRequired(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	value, err := promptDefault(reader, out, label, "")
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("%s is required", strings.ToLower(label))
	}
	return value, nil
}

func promptDefault(reader *bufio.Reader, out io.Writer, label, fallback string) (string, error) {
	if fallback == "" {
		fmt.Fprintf(out, "%s: ", label)
	} else {
		fmt.Fprintf(out, "%s [%s]: ", label, fallback)
	}
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" && errors.Is(err, io.EOF) {
		return "", io.EOF
	}
	if value == "" {
		return fallback, nil
	}
	return value, nil
}

func runGit(args []string) error {
	fs := flag.NewFlagSet("git", flag.ContinueOnError)
	repository := fs.String("repo", ".", "Git repository path")
	message := fs.String("message", "", "Commit message (generated when empty)")
	dryRun := fs.Bool("dry-run", false, "Show the generated commit message without changing Git")
	if len(args) == 0 {
		return errors.New("usage: routerctl git status|commit|sync [--repo <path>] [--message <message>] [--dry-run]")
	}
	command := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	options := gitops.Options{Repository: *repository, Message: *message, DryRun: *dryRun}
	switch command {
	case "status":
		if *message != "" || *dryRun {
			return errors.New("git status: --message and --dry-run are not supported")
		}
		status, err := gitops.StatusAt(*repository)
		if err != nil {
			return err
		}
		return printJSON(os.Stdout, status)
	case "commit":
		commitMessage, err := gitops.Commit(options)
		if err != nil {
			return err
		}
		if commitMessage == "" {
			fmt.Println("clean")
		} else {
			fmt.Println(commitMessage)
		}
		return nil
	case "sync":
		commitMessage, err := gitops.Sync(options)
		if err != nil {
			return err
		}
		if commitMessage == "" {
			fmt.Println("synced (no local changes)")
		} else {
			fmt.Printf("synced: %s\n", commitMessage)
		}
		return nil
	default:
		return fmt.Errorf("unknown git command %q", command)
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
	if len(args) == 22 && args[0] == "bundle" && args[1] == "create" {
		values := map[string]string{}
		for i := 2; i < len(args); i += 2 {
			values[args[i]] = args[i+1]
		}
		bundle, err := regulatory.CreateBundle(values["--authority"], values["--number"], values["--source-url"], values["--checked-at"], values["--checked-by"], values["--number-evidence"], values["--report"], values["--vendor"], values["--model"], values["--revision"])
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
  routerctl interactive                 # guided mode for every command
  routerctl --interactive               # same as above
  routerctl version
	  routerctl git status [--repo <path>]
	  routerctl git commit [--repo <path>] [--message <message>] [--dry-run]
	  routerctl git sync [--repo <path>] [--message <message>] [--dry-run]
  routerctl inspect <manifest>
  routerctl plan <manifest>
  routerctl verify <manifest>
  routerctl resolve <manifest>
  routerctl verify-release <manifest.json> <SHA256SUMS> <provenance.json>
  routerctl regulatory import mic <document.pdf|document.txt>
  routerctl regulatory import mic --bundle <bundle.json>
  routerctl regulatory bundle create --authority MIC --number <number> --source-url <exact-url> --checked-at <RFC3339> --checked-by <reviewer> --number-evidence <file> --report <file> --vendor <vendor> --model <model> --revision <revision>
  routerctl regulatory validate <record.json>
  routerctl regulatory derive <record.json>
  routerctl regulatory explain <derived.json> <key>
  routerctl regulatory profile check <profile.yaml>
  routerctl regulatory label extract --image <image> --layout <layout.yaml> --out <dir>
  routerctl regulatory label verify <bundle-dir>`)
}

const maxManifestCandidates = 8

// nextSteps is the compact, task-oriented entry point shown when routerctl is
// invoked without a command. It deliberately only suggests routerctl Device
// manifests, rather than every YAML file in a workspace.
func nextSteps(w io.Writer) {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(w, "routerctl: choose a device manifest, then verify it:")
		fmt.Fprintln(w, "  routerctl verify <manifest>")
		return
	}
	nextStepsAt(w, root)
}

func nextStepsAt(w io.Writer, root string) {
	candidates := findDeviceManifests(root)
	fmt.Fprintln(w, "routerctl: start by checking a device manifest.")
	if len(candidates) == 0 {
		fmt.Fprintln(w, "No device manifests found below the current directory.")
		fmt.Fprintln(w, "  routerctl verify <manifest>")
		fmt.Fprintln(w, "  routerctl plan <manifest>")
		fmt.Fprintln(w, "Run `routerctl --help` to see every command.")
		return
	}

	fmt.Fprintln(w, "Device manifest candidates:")
	for _, candidate := range candidates {
		fmt.Fprintf(w, "  %s (%s)\n", candidate.Path, candidate.Name)
	}
	first := candidates[0].Path
	fmt.Fprintln(w, "Next:")
	fmt.Fprintf(w, "  routerctl verify %s\n", first)
	fmt.Fprintf(w, "  routerctl plan %s\n", first)
	fmt.Fprintln(w, "Run `routerctl --help` to see every command.")
}

type manifestCandidate struct {
	Path string
	Name string
}

func findDeviceManifests(root string) []manifestCandidate {
	var candidates []manifestCandidate
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && ignoredManifestDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".yaml" && extension != ".yml" {
			return nil
		}
		m, loadErr := manifest.Load(path)
		if loadErr != nil || m.APIVersion != "routerctl.dev/v1alpha1" || m.Kind != "Device" {
			return nil
		}
		relativePath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		candidates = append(candidates, manifestCandidate{Path: relativePath, Name: m.Metadata.Name})
		return nil
	})
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Path < candidates[j].Path })
	if len(candidates) > maxManifestCandidates {
		return candidates[:maxManifestCandidates]
	}
	return candidates
}

func ignoredManifestDirectory(name string) bool {
	return strings.HasPrefix(name, ".") || name == "packaging" || name == "vendor" || name == "node_modules"
}
