#!/usr/bin/env bash
set -euo pipefail

# Preserve the previously deployed package without rebuilding or relabeling it.
if [ "$#" -ne 2 ]; then
  echo 'Usage: retain_miniapp_artifact.sh <current-app> <release-app>' >&2
  exit 2
fi
previous="$1/miniapp/dist/build/mp-weixin"
incoming="$2/miniapp/dist/build/mp-weixin"
for required in app.json project.config.json RELEASE_INFO PAGE_FILE_MANIFEST; do
  if [ ! -f "$previous/$required" ]; then
    echo "ERROR: cannot retain existing miniapp artifact; missing $required" >&2
    exit 1
  fi
done
if [ ! -d "$previous/pages" ]; then
  echo 'ERROR: cannot retain existing miniapp artifact; missing pages directory' >&2
  exit 1
fi
if find "$previous" -type l -print -quit | grep -q .; then
  echo 'ERROR: cannot verify miniapp artifact containing symbolic links' >&2
  exit 1
fi
if [ -e "$incoming" ]; then
  echo 'ERROR: refusing to overwrite a miniapp artifact in the release source' >&2
  exit 1
fi
hash_dir="$(mktemp -d)"
trap 'rm -rf "$hash_dir"' EXIT
( cd "$previous"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum ) > "$hash_dir/before"
mkdir -p "$incoming"
tar -C "$previous" -cf - . | tar -C "$incoming" -xf -
( cd "$incoming"; find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum ) > "$hash_dir/after"
if ! cmp -s "$hash_dir/before" "$hash_dir/after"; then
  echo 'ERROR: retained miniapp artifact failed SHA-256 verification' >&2
  exit 1
fi
echo 'Retained existing miniapp artifact unchanged; it is not built for this release.'
