#!/usr/bin/env bash
set -euo pipefail

profile=${1:?usage: packaging/build.sh <profile>}
repo_root=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
upstream="$repo_root/upstream/firmware"
profile_dir="$repo_root/profiles/$profile"
patch_dir="$repo_root/patches/$profile"

"$repo_root/packaging/check-layer.sh" "$profile"
# shellcheck disable=SC1090
source "$profile_dir/profile.env"
: "${UPSTREAM_BUILD_COMMAND:?profile must set UPSTREAM_BUILD_COMMAND}"

build_root=$(mktemp -d "${TMPDIR:-/tmp}/routerctl-${profile}.XXXXXX")
cleanup() {
  git -C "$upstream" worktree remove --force "$build_root/source" >/dev/null 2>&1 || true
  rm -rf "$build_root"
}
trap cleanup EXIT

git -C "$upstream" worktree add --detach "$build_root/source" HEAD >/dev/null

if [[ -d "$patch_dir" ]]; then
  shopt -s nullglob
  patches=("$patch_dir"/*.patch)
  if ((${#patches[@]})); then
    git -C "$build_root/source" am --3way "${patches[@]}"
  fi
fi

(
  cd "$build_root/source"
  eval "$UPSTREAM_BUILD_COMMAND"
)

mkdir -p "$repo_root/dist/$profile"
if [[ -n "${ARTIFACT_GLOB:-}" ]]; then
  shopt -s globstar nullglob
  artifacts=("$build_root/source"/$ARTIFACT_GLOB)
  ((${#artifacts[@]})) || { echo "no artifacts matched: $ARTIFACT_GLOB" >&2; exit 1; }
  cp -- "${artifacts[@]}" "$repo_root/dist/$profile/"
fi
(cd "$repo_root/dist/$profile" && sha256sum ./* > SHA256SUMS)
echo "artifacts: $repo_root/dist/$profile"
