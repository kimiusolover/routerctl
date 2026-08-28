# routerctl

Cross-platform host-side orchestrator for router OS development and deployment.

## tiny-layer policy

This repository is an integration layer, not a fork of firmware upstream.  The
firmware source lives at `upstream/firmware` as a Git submodule and must remain
at an upstream commit with no local edits.  All product-specific work belongs
to one of the parent-repository layers:

- `profiles/` — declarative build settings, feeds, configs, and target choices
- `patches/` — ordered, reviewable downstream commits
- `packaging/` — reproducible build and release assembly

`packaging/build.sh` creates a detached temporary worktree of the submodule,
applies the patch series there, and runs the selected profile.  Consequently,
neither building nor packaging dirties `upstream/firmware`.

### Add an upstream

Choose and pin the exact source and revision, then register it once:

```sh
git submodule add <upstream-url> upstream/firmware
git -C upstream/firmware checkout <upstream-ref-or-commit>
git add .gitmodules upstream/firmware
```

For a fresh clone, initialize it with:

```sh
git submodule update --init --recursive
```

See [`layer/README.md`](layer/README.md) for the layer contract, patch
workflow, and release procedure.

`routerctl` deliberately keeps firmware/image generation outside the CLI core.
The intended flow is:

    manifest -> plan -> backend -> artifact -> transport -> device

## Initial commands

```sh
go run ./cmd/routerctl version
go run ./cmd/routerctl inspect examples/ax23v/device.yaml
go run ./cmd/routerctl plan examples/ax23v/device.yaml
go run ./cmd/routerctl verify examples/ax23v/device.yaml
go run ./cmd/routerctl resolve examples/ax23v/device.yaml
go run ./cmd/routerctl verify-release manifest.json SHA256SUMS provenance.json
go run ./cmd/routerctl regulatory import mic examples/ax23v/regulatory/JP/mic-report.txt
```

## Regulatory evidence workflow

A person searches the official authority database and selects a local document.
`routerctl` imports that document without automating the authority search UI or
bypassing its access controls. Imported values retain the document SHA-256 and
page evidence; they are observations, not firmware settings.

```sh
routerctl regulatory import mic report.pdf > record.json
routerctl regulatory validate record.json
routerctl regulatory derive record.json > derived.json
routerctl regulatory explain derived.json wifi.5GHz.max_tx_power_dbm
```

Only an observation explicitly classified as `certified_max` can enter the
minimal derivation rule. Conducted-power, EIRP, measured, and configured values
are evidence only and are never turned into a firmware limit by inference.

## Architecture

- `internal/manifest` — device manifest loading and validation
- `internal/planner` — converts a manifest into an execution plan
- `internal/backend` — build/artifact backend abstraction
- `internal/backend/github` — GitHub Releases artifact resolver
- `internal/transport` — device transport abstraction
- `internal/verify` — host-side manifest verification
- `internal/cli` — command dispatch
- `internal/regulatory` — evidence-preserving certification document import
- `schemas` — machine-readable manifest schemas
- `examples` — device examples

Backends and transports are interfaces so the core does not depend on a
specific host operating system, firmware repository, or deployment mechanism.

## Workspace context

Workspace discovery is deliberately independent from Git discovery. A
workspace has a `.routerctl/workspace.json` marker; an optional
`.routerctl/components.json` registry assigns paths to components:

```json
{
  "components": [
    {"name": "firmware", "root": "upstream/firmware"},
    {"name": "profiles", "root": "profiles"}
  ]
}
```

The context resolver normalizes the input path, finds the workspace marker,
then detects the enclosing Git repository and its superproject (when the
repository is a submodule). It selects the deepest registered component that
contains the file and returns a component-relative path only after checking the
component boundary. Thus Git root, workspace root, component root, and current
directory remain separate values.

## router-firmware release connection

Set `github.repository` in the device manifest to the GitHub repository that
publishes `router-firmware` releases (for example, `your-org/router-firmware`).
The release must contain the exact asset named by `spec.target`, such as
`ax23v-v1.bin`. Optionally pin `github.tag`; an empty value uses the latest
release. `routerctl resolve` returns the download URL and GitHub-provided digest
without downloading or flashing the artifact.

## Status

## Release metadata verification

Each release publishes three independent views of every binary: `manifest.json`,
`SHA256SUMS`, and an in-toto `provenance.json` statement. Verify all three
before using a release:

```sh
routerctl verify-release manifest.json SHA256SUMS provenance.json
```

The command requires exactly the same artifact names and SHA-256 digests in all
three inputs. It accepts either a plain in-toto Statement or a DSSE envelope
whose payload is that statement, and rejects missing, duplicate, malformed, or
mismatched entries.

## Regulatory evidence import

`routerctl regulatory import mic <document>` turns a locally obtained MIC
test report into a JSON certification record. It never queries official
search sites; an operator obtains the certification number and document first.
The record preserves the source filename, source SHA-256, page, and the
meaning of each value. In particular, conducted test power is not treated as a
firmware setting. PDF input uses the locally installed `pdftotext`; the AX23v
fixture is extracted text so tests stay deterministic.

The AX23v fixture is deliberately synthetic, not an actual certification
record. Replace it with the operator-downloaded official report before using
any result for a build constraint.

Bootstrap repository. `build`, `deploy`, `rollback`, `health`, and
`collect-logs` are reserved for later implementation.
