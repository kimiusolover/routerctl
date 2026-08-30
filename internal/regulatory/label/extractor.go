package label

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	MaxImageFileSize = 50 * 1024 * 1024 // 50MB
	MaxDimension     = 10000
	MinDimension     = 10
)

var (
	GeometryRegex = regexp.MustCompile(`^(\d+)x(\d+)\+(\d+)\+(\d+)$`)

	pngMagic  = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	jpegMagic = []byte{0xff, 0xd8, 0xff}

	dangerousPrefixes = []string{
		"http:", "https:", "ftp:", "file:", "ephemeral:", "null:", "caption:",
		"label:", "inline:", "mrf:", "pattern:", "gradient:", "plasma:",
		"vid:", "fd:", "xc:",
	}
)

type CropTarget struct {
	Role     string
	Geometry string
}

type ImageDimensions struct {
	Width  int
	Height int
}

type ExtractOptions struct {
	SourcePath   string
	SourceSHA256 string
	LayoutID     string
	FinalDir     string
	Crops        []CropTarget
	Reviewed     ReviewedSpec
	RunOCR       bool
	OCROptional  bool
	OCRLang      string
	OCRPattern   *regexp.Regexp
	MinConf      float64
}

// ValidateImageSource validates that the source file is a local regular file of allowed format.
func ValidateImageSource(srcPath string) error {
	if srcPath == "" {
		return fmt.Errorf("source image path must not be empty")
	}

	cleanPath := filepath.Clean(srcPath)
	lowerPath := strings.ToLower(cleanPath)

	for _, prefix := range dangerousPrefixes {
		if strings.HasPrefix(lowerPath, prefix) {
			return fmt.Errorf("source image path uses prohibited protocol or delegate prefix: %q", prefix)
		}
	}

	if strings.Contains(srcPath, "\x00") {
		return fmt.Errorf("source image path contains invalid null bytes")
	}

	info, err := os.Lstat(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to stat source image: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("source image must not be a symbolic link: %s", cleanPath)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("source image is not a regular file: %s", cleanPath)
	}

	if info.Size() > MaxImageFileSize {
		return fmt.Errorf("source image size (%d bytes) exceeds maximum limit (%d bytes)", info.Size(), MaxImageFileSize)
	}

	f, err := os.Open(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to open source image: %w", err)
	}
	defer f.Close()

	header := make([]byte, 16)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read source image header: %w", err)
	}
	if n < 3 {
		return fmt.Errorf("source image is too small to identify format")
	}

	isPNG := bytes.HasPrefix(header[:n], pngMagic)
	isJPEG := bytes.HasPrefix(header[:n], jpegMagic)

	if !isPNG && !isJPEG {
		return fmt.Errorf("unsupported image format (only PNG and JPEG are supported, magic bytes mismatch)")
	}

	return nil
}

func GetImageDimensions(ctx context.Context, imgPath string) (ImageDimensions, error) {
	cmd := exec.CommandContext(ctx, "magick",
		"identify",
		"-limit", "memory", "256MiB",
		"-limit", "map", "512MiB",
		"-limit", "disk", "1GiB",
		"-limit", "time", "30",
		"-format", "%w %h",
		imgPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback to standalone identify binary, strictly preserving the exact same resource limits
		cmdFallback := exec.CommandContext(ctx, "identify",
			"-limit", "memory", "256MiB",
			"-limit", "map", "512MiB",
			"-limit", "disk", "1GiB",
			"-limit", "time", "30",
			"-format", "%w %h",
			imgPath,
		)
		var errFallback error
		out, errFallback = cmdFallback.CombinedOutput()
		if errFallback != nil {
			return ImageDimensions{}, fmt.Errorf("failed to identify image dimensions with resource limits: %w (fallback: %v, output: %s)", err, errFallback, string(out))
		}
	}

	parts := strings.Fields(string(out))
	if len(parts) != 2 {
		return ImageDimensions{}, fmt.Errorf("unexpected identify output: %s", string(out))
	}

	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return ImageDimensions{}, fmt.Errorf("invalid dimensions returned: %s", string(out))
	}

	if w < MinDimension || h < MinDimension || w > MaxDimension || h > MaxDimension {
		return ImageDimensions{}, fmt.Errorf("image dimensions %dx%d out of supported bounds (%d-%d px)", w, h, MinDimension, MaxDimension)
	}

	return ImageDimensions{Width: w, Height: h}, nil
}

func ExtractAndCommitBundle(
	ctx context.Context,
	srcPath string,
	srcSHA256 string,
	layoutID string,
	finalDir string,
	crops []CropTarget,
) (*RegulatoryBundle, error) {
	return ExtractAndCommitBundleWithOptions(ctx, ExtractOptions{
		SourcePath:   srcPath,
		SourceSHA256: srcSHA256,
		LayoutID:     layoutID,
		FinalDir:     finalDir,
		Crops:        crops,
		RunOCR:       false,
	})
}

func ExtractAndCommitBundleWithOptions(
	ctx context.Context,
	opts ExtractOptions,
) (*RegulatoryBundle, error) {
	if opts.SourcePath == "" {
		return nil, fmt.Errorf("source path must not be empty")
	}
	if opts.FinalDir == "" {
		return nil, fmt.Errorf("destination directory must not be empty")
	}
	if len(opts.Crops) == 0 {
		return nil, fmt.Errorf("at least one crop target must be specified")
	}

	// 1. Validate source image security and format
	if err := ValidateImageSource(opts.SourcePath); err != nil {
		return nil, fmt.Errorf("invalid source image: %w", err)
	}

	// 2. Check destination directory collision
	if _, err := os.Lstat(opts.FinalDir); err == nil {
		return nil, fmt.Errorf("destination directory already exists: %s", opts.FinalDir)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to check destination directory: %w", err)
	}

	// 3. Compute and verify source SHA-256 digest
	computedSHA256, err := calculateSHA256(opts.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute source hash: %w", err)
	}

	if opts.SourceSHA256 != "" && !strings.EqualFold(opts.SourceSHA256, computedSHA256) {
		return nil, fmt.Errorf("source SHA-256 digest mismatch: expected %s, got %s", opts.SourceSHA256, computedSHA256)
	}

	// 4. Validate roles and bounding box geometry
	imgDim, err := GetImageDimensions(ctx, opts.SourcePath)
	if err != nil {
		return nil, err
	}

	seenRoles := make(map[string]bool)
	var validatedCrops []CropTarget

	for _, c := range opts.Crops {
		if !AllowedRoles[c.Role] {
			return nil, fmt.Errorf("unauthorized or invalid role: %s", c.Role)
		}
		if seenRoles[c.Role] {
			return nil, fmt.Errorf("duplicate role specified: %s", c.Role)
		}
		seenRoles[c.Role] = true

		matches := GeometryRegex.FindStringSubmatch(c.Geometry)
		if len(matches) != 5 {
			return nil, fmt.Errorf("invalid geometry format for %s: %s (must be WxH+X+Y)", c.Role, c.Geometry)
		}

		w, _ := strconv.Atoi(matches[1])
		h, _ := strconv.Atoi(matches[2])
		x, _ := strconv.Atoi(matches[3])
		y, _ := strconv.Atoi(matches[4])

		if w <= 0 || h <= 0 {
			return nil, fmt.Errorf("crop %s has invalid non-positive dimensions (%dx%d)", c.Role, w, h)
		}

		if x+w > imgDim.Width || y+h > imgDim.Height {
			return nil, fmt.Errorf("crop %s (%s) exceeds image boundary (%dx%d)", c.Role, c.Geometry, imgDim.Width, imgDim.Height)
		}

		validatedCrops = append(validatedCrops, c)
	}

	// Enforce all required roles
	for _, req := range RequiredRoles {
		if !seenRoles[req] {
			return nil, fmt.Errorf("missing mandatory crop role: %q (required roles: %v)", req, RequiredRoles)
		}
	}

	// Deterministic sorting by role name
	sort.Slice(validatedCrops, func(i, j int) bool {
		return validatedCrops[i].Role < validatedCrops[j].Role
	})

	// 5. Create staging directory inside parent directory of finalDir (preventing EXDEV)
	parentDir := filepath.Dir(filepath.Clean(opts.FinalDir))
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to ensure parent directory: %w", err)
	}

	stagingDir, err := os.MkdirTemp(parentDir, ".staging-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create staging directory in %s: %w", parentDir, err)
	}
	defer os.RemoveAll(stagingDir)

	// 6. Invoke ImageMagick with resource limits to produce PNG crops
	args := []string{
		"-limit", "memory", "256MiB",
		"-limit", "map", "512MiB",
		"-limit", "disk", "1GiB",
		"-limit", "time", "30",
		opts.SourcePath,
	}
	for _, c := range validatedCrops {
		stagedFile := filepath.Join(stagingDir, c.Role+".png")
		args = append(args, "(", "-clone", "0", "-crop", c.Geometry, "+repage", "-write", stagedFile, "+delete", ")")
	}
	args = append(args, "null:")

	cmd := exec.CommandContext(ctx, "magick", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("magick execution failed (%v): %s", err, string(out))
	}

	// 7. Compute SHA-256 for all artifacts
	var artifacts []ArtifactSpec
	for _, c := range validatedCrops {
		stagedFile := filepath.Join(stagingDir, c.Role+".png")
		hash, err := calculateSHA256(stagedFile)
		if err != nil {
			return nil, fmt.Errorf("failed to compute digest for %s: %w", c.Role, err)
		}
		artifacts = append(artifacts, ArtifactSpec{
			Role:   c.Role,
			File:   c.Role + ".png",
			SHA256: hash,
		})
	}

	bundle := &RegulatoryBundle{
		Version:     "1.0",
		GeneratedAt: time.Now().UTC(),
		Source: SourceSpec{
			SHA256:   computedSHA256,
			LayoutID: opts.LayoutID,
		},
		Artifacts: artifacts,
	}

	// 8. OCR candidate extraction & status tracking
	if opts.RunOCR {
		pattern := opts.OCRPattern
		if pattern == nil {
			pattern = regexp.MustCompile(`\d{3}-\d{6}`)
		}
		minConf := opts.MinConf
		if minConf <= 0 {
			minConf = 60.0
		}
		lang := opts.OCRLang
		if lang == "" {
			lang = "eng"
		}

		gitekiCropFile := filepath.Join(stagingDir, "giteki_mark_and_number_crop.png")
		candidates, err := ExtractCertificationNumbersWithLang(ctx, gitekiCropFile, lang, pattern, minConf)
		if err != nil {
			if !opts.OCROptional {
				return nil, fmt.Errorf("OCR candidate extraction failed: %w (use --ocr-optional to proceed without OCR)", err)
			}
			bundle.Observations.OCRCandidates = OCRSpec{
				Status: "unavailable",
				Error:  err.Error(),
			}
		} else if len(candidates) > 0 {
			bundle.Observations.OCRCandidates = OCRSpec{
				Status:               "success",
				CertificationNumbers: candidates,
			}
		} else {
			bundle.Observations.OCRCandidates = OCRSpec{
				Status: "no_match",
			}
		}
	} else {
		bundle.Observations.OCRCandidates = OCRSpec{
			Status: "not_run",
		}
	}

	if opts.Reviewed.Vendor != "" || opts.Reviewed.Model != "" || opts.Reviewed.HardwareRevision != "" || opts.Reviewed.CertificationNumber != "" || opts.Reviewed.Status != "" {
		bundle.Observations.Reviewed = opts.Reviewed
	}

	// 9. Validate bundle against schema integrity rules
	if err := bundle.Validate(); err != nil {
		return nil, fmt.Errorf("bundle validation failed: %w", err)
	}

	// 10. Write bundle manifest
	manifestPath := filepath.Join(stagingDir, "bundle.yaml")
	manifestData, err := yaml.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bundle manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write bundle manifest: %w", err)
	}

	// 11. Atomic commit by renaming directory
	if err := os.Rename(stagingDir, opts.FinalDir); err != nil {
		return nil, fmt.Errorf("failed to commit staging bundle to final location: %w", err)
	}

	return bundle, nil
}

// VerifyBundleDirectory verifies that a bundle directory contains a valid bundle.yaml,
// all referenced artifact files exist, no symlinks exist, paths are strictly contained,
// and on-disk SHA-256 digests match exactly.
func VerifyBundleDirectory(bundleDir string) (*RegulatoryBundle, error) {
	cleanDir := filepath.Clean(bundleDir)
	info, err := os.Lstat(cleanDir)
	if err != nil {
		return nil, fmt.Errorf("failed to access bundle directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("bundle directory must not be a symbolic link: %s", cleanDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", cleanDir)
	}

	manifestPath := filepath.Join(cleanDir, "bundle.yaml")
	manInfo, err := os.Lstat(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access bundle manifest: %w", err)
	}
	if manInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("bundle.yaml must not be a symbolic link: %s", manifestPath)
	}
	if !manInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("bundle.yaml is not a regular file: %s", manifestPath)
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle manifest: %w", err)
	}

	var bundle RegulatoryBundle
	if err := yaml.Unmarshal(manifestBytes, &bundle); err != nil {
		return nil, fmt.Errorf("failed to parse bundle manifest YAML: %w", err)
	}

	if err := bundle.Validate(); err != nil {
		return nil, fmt.Errorf("bundle manifest validation failed: %w", err)
	}

	expectedFiles := map[string]bool{
		"bundle.yaml": true,
	}

	for _, art := range bundle.Artifacts {
		// Strict path containment checks
		cleanedFile := filepath.Clean(art.File)
		if cleanedFile != art.File || filepath.IsAbs(cleanedFile) || strings.Contains(cleanedFile, "..") || strings.Contains(cleanedFile, string(filepath.Separator)) {
			return nil, fmt.Errorf("artifact file path %q violates directory containment", art.File)
		}

		artPath := filepath.Join(cleanDir, cleanedFile)
		artInfo, err := os.Lstat(artPath)
		if err != nil {
			return nil, fmt.Errorf("artifact file %q missing from bundle: %w", art.File, err)
		}
		if artInfo.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("artifact file %q must not be a symbolic link", art.File)
		}
		if !artInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("artifact %q is not a regular file", art.File)
		}

		computedHash, err := calculateSHA256(artPath)
		if err != nil {
			return nil, fmt.Errorf("failed to compute digest for artifact %q: %w", art.File, err)
		}

		if !strings.EqualFold(computedHash, art.SHA256) {
			return nil, fmt.Errorf("digest mismatch for artifact %q: expected %s, got %s", art.File, art.SHA256, computedHash)
		}

		expectedFiles[cleanedFile] = true
	}

	// Verify no extraneous, symlinked, or unauthorized files exist in the bundle
	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle directory entries: %w", err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(cleanDir, entry.Name())
		entryInfo, err := os.Lstat(entryPath)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect entry %q: %w", entry.Name(), err)
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unauthorized symbolic link in bundle directory: %s", entry.Name())
		}
		if !entryInfo.Mode().IsRegular() {
			return nil, fmt.Errorf("unauthorized non-regular entry in bundle directory: %s", entry.Name())
		}
		if !expectedFiles[entry.Name()] {
			return nil, fmt.Errorf("extraneous unauthorized file in bundle: %s", entry.Name())
		}
	}

	return &bundle, nil
}

func calculateSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
