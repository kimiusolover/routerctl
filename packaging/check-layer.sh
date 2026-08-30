#!/usr/bin/env bash
set -euo pipefail

profile=${1:?usage: packaging/check-layer.sh <profile>}
repo_root=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
upstream="$repo_root/upstream/firmware"
profile_file="$repo_root/profiles/$profile/profile.env"

[[ -f "$profile_file" ]] || { echo "profile not found: $profile" >&2; exit 1; }
git -C "$repo_root" submodule status -- upstream/firmware 2>/dev/null | grep -Eq '^[ +-]?[0-9a-f]{40} ' || {
  echo "upstream/firmware is not an initialized Git submodule" >&2
  exit 1
}
git -C "$upstream" rev-parse --is-inside-work-tree >/dev/null
[[ -z "$(git -C "$upstream" status --porcelain)" ]] || {
  echo "upstream/firmware must be clean; place changes in patches/" >&2
  exit 1
}
echo "tiny-layer checks passed for profile: $profile"
