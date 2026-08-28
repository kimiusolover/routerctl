# tiny-layer contract

The parent repository owns product policy.  `upstream/firmware` is a Git
submodule that records an unmodified upstream commit; do not commit source
changes inside it.

## Directory ownership

| Location | Owns | Must not contain |
| --- | --- | --- |
| `upstream/firmware` | upstream source at a pinned commit | local source changes |
| `profiles/<name>/` | target selection, config fragments, feeds, metadata | copied upstream code |
| `patches/<name>/` | ordered `.patch` series applied after checkout | generated binaries |
| `packaging/` | isolated build, artifact naming and checksums | product source forks |

## Profile contract

Each profile contains a `profile.env` file. It is sourced only by the local
packaging script and defines:

```sh
UPSTREAM_BUILD_COMMAND='make defconfig world'
ARTIFACT_GLOB='bin/targets/**/*.bin'
```

The command runs from the temporary worktree.  Keep profile values declarative;
when settings require source changes, express those changes as patches.

## Patches

Patch file names are applied in lexical order.  Generate patches from commits
made in a disposable worktree, then store them under the relevant profile:

```sh
git -C /path/to/disposable-worktree format-patch --no-signature \
  --output-directory patches/<profile> <pinned-upstream>..HEAD
```

`packaging/build.sh` uses `git am --3way`, so a failed rebase stops without
touching the submodule. Update the patch series when moving the submodule pin.

## Invariants checked before a build

- `upstream/firmware` is an initialized Git submodule.
- Its working tree is clean.
- The selected profile has `profile.env`.
- Every patch applies to a detached build worktree, never to the submodule.

Run `packaging/check-layer.sh <profile>` for the first three checks, or
`packaging/build.sh <profile>` to perform the isolated build.
