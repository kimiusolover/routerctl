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
- `devices/` — product hardware definitions and narrowly scoped overlays

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

Run `routerctl` from a terminal with no arguments to start its guided mode.
You can also request it explicitly (including when input is piped):

```sh
routerctl interactive
# or: routerctl --interactive
```

The interactive shell keeps running after each command, like `parted`:

```text
[routerctl]# verify examples/ax23v/device.yaml
[routerctl]# plan
Manifest path: examples/ax23v/device.yaml
[routerctl]# reg<Tab>
[routerctl]# help
[routerctl]# quit
```

Tab completes top-level commands, the `git` / `regulatory` command tree, and
device manifests. A second Tab lists ambiguous candidates; `Ctrl+D` exits. It
asks for omitted required values and asks for a final confirmation before a Git
commit or sync. The ordinary command forms below remain available for scripts
and CI.

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
routerctl regulatory profile check examples/ax23v/regulatory/JP/certification-profile.yaml
```

Only an observation explicitly classified as `certified_max` can enter the
minimal derivation rule. Conducted-power, EIRP, measured, and configured values
are evidence only and are never turned into a firmware limit by inference.

`CertificationProfile` v1 is a separate, fail-closed policy input. It records
the statutory ceiling, certified rated limit, hardware identity, DFS
requirements, and source references separately. The runtime limit must be the
minimum of statutory, certified, detected-hardware/calibration, and driver
limits; measured test output is evidence only. A profile with
`evidenceStatus: incomplete` is useful for review but cannot authorize RF
transmission. Check a profile with:

```sh
routerctl regulatory profile check examples/ax23v/regulatory/JP/certification-profile.yaml
```

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
- `devices` — layered hardware and OpenWrt compatibility definitions
- `examples` — device examples

Backends and transports are interfaces so the core does not depend on a
specific host operating system, firmware repository, or deployment mechanism.

## Local Git workflow

`routerctl git` keeps local commits and pushes repeatable without granting the
tool permission to rewrite history. It generates a conservative Conventional
Commit message from changed paths; supply `--message` whenever a more specific
message is needed.

```sh
# Inspect the current checkout.
routerctl git status

# Preview the generated message. No files are staged or committed.
routerctl git commit --dry-run

# Stage local changes and create one commit.
routerctl git commit

# Commit if needed, pull with rebase, then make a normal push.
routerctl git sync
```

`sync` requires an upstream branch and stops if rebase conflicts occur. It
never runs `push --force`, `reset`, or `commit --amend`; paths that look like
private keys, credentials, secrets, or `.env` files are refused before staging.

## Install

Install the CLI into your user-local binary directory, then invoke it from any
directory as `routerctl`:

```sh
make install
routerctl version
```

The default destination is `~/.local/bin`. If that directory is not on your
shell `PATH`, add this line to the relevant shell startup file and open a new
terminal:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

For a different user-local prefix, use `make install PREFIX=/your/prefix`.

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

## Label evidence extraction

`routerctl regulatory label extract` accepts a reviewed YAML label layout and
emits only the device identity and the combined technical-conformity mark plus
number crop. It does not retain the full physical-label photograph in the
bundle, and it does not run OCR by default. The resulting crop is evidence that
the mark and number appeared together on a physical label; official
certificate and test-report documents remain the primary certification
evidence.

```sh
routerctl regulatory label extract \
  --image device-label.jpg \
  --layout examples/ax23v/regulatory/JP/label-layout.yaml \
  --out /tmp/ax23v-label-evidence
```

Bootstrap repository. `build`, `deploy`, `rollback`, `health`, and
`collect-logs` are reserved for later implementation.
