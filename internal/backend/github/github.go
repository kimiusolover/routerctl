// Package github resolves release assets published through GitHub Releases.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/example/routerctl/internal/backend"
)

const defaultAPIURL = "https://api.github.com"

// Config identifies the release from which artifacts are resolved.
// Repository must be in the form "owner/repository".  When Tag is empty,
// Resolve uses the repository's latest release.
type Config struct {
	Repository string
	Tag        string
	Token      string
	APIURL     string
	HTTPClient *http.Client
}

// Backend resolves an asset name (Request.Target) to its GitHub download URL.
type Backend struct {
	repository string
	tag        string
	token      string
	apiURL     string
	client     *http.Client
}

// New creates a GitHub Releases backend. It validates configuration eagerly so
// deployment configuration errors are reported before a network request.
func New(config Config) (*Backend, error) {
	parts := strings.Split(config.Repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("github backend: repository must be owner/repository, got %q", config.Repository)
	}

	apiURL := config.APIURL
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	parsedURL, err := url.Parse(apiURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("github backend: invalid API URL %q", apiURL)
	}

	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	return &Backend{
		repository: config.Repository,
		tag:        config.Tag,
		token:      config.Token,
		apiURL:     strings.TrimRight(apiURL, "/"),
		client:     client,
	}, nil
}

func (*Backend) Name() string { return "github" }

// Resolve fetches the configured release and finds the asset named by
// req.Target. Artifact.Path is the asset's direct download URL. GitHub's
// optional asset digest is returned unchanged (normally sha256:<hex>).
func (b *Backend) Resolve(ctx context.Context, req backend.Request) (backend.Artifact, error) {
	if strings.TrimSpace(req.Target) == "" {
		return backend.Artifact{}, errors.New("github backend: target (release asset name) is required")
	}

	endpoint := b.apiURL + "/repos/" + b.repository + "/releases/latest"
	if b.tag != "" {
		endpoint = b.apiURL + "/repos/" + b.repository + "/releases/tags/" + url.PathEscape(b.tag)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return backend.Artifact{}, fmt.Errorf("github backend: create release request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	if b.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.token)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return backend.Artifact{}, fmt.Errorf("github backend: get release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return backend.Artifact{}, fmt.Errorf("github backend: get release: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var release release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return backend.Artifact{}, fmt.Errorf("github backend: decode release: %w", err)
	}
	for _, asset := range release.Assets {
		if asset.Name == req.Target {
			if asset.BrowserDownloadURL == "" {
				return backend.Artifact{}, fmt.Errorf("github backend: asset %q has no download URL", req.Target)
			}
			return backend.Artifact{Path: asset.BrowserDownloadURL, Digest: asset.Digest}, nil
		}
	}
	return backend.Artifact{}, fmt.Errorf("github backend: release asset %q not found in %s", req.Target, b.repository)
}

type release struct {
	Assets []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}
