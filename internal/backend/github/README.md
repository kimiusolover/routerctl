# GitHub backend

Resolves a named artifact from a GitHub Release.

Create it with a repository (`owner/repository`) and, optionally, a release tag.
An empty tag selects the latest release. `backend.Request.Target` is the exact
release asset name; the returned artifact path is its download URL. Supply a
GitHub token in `Config.Token` for private repositories or higher API limits.

```go
resolver, err := github.New(github.Config{
    Repository: "acme/router-firmware",
    Tag:        "v1.2.0",
    Token:      os.Getenv("GITHUB_TOKEN"),
})
artifact, err := resolver.Resolve(ctx, backend.Request{Target: "ax23v.bin"})
```

`routerctl` reads the same configuration from a device manifest and resolves it
with `routerctl resolve <manifest>`. Set `GITHUB_TOKEN` only for a private
repository or to raise GitHub API limits; do not place tokens in the manifest.
