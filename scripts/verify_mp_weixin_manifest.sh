#!/usr/bin/env bash
set -euo pipefail

ARTIFACT_DIR="${1:-}"
if [ -z "$ARTIFACT_DIR" ] || [ ! -d "$ARTIFACT_DIR" ]; then
  echo "ERROR: mp-weixin artifact directory is required" >&2
  exit 1
fi

MANIFEST="$ARTIFACT_DIR/PAGE_FILE_MANIFEST"
if [ ! -f "$MANIFEST" ]; then
  echo "ERROR: mp-weixin artifact page manifest is missing: PAGE_FILE_MANIFEST" >&2
  exit 1
fi

artifact_file=""
file_count=0
while IFS= read -r artifact_file || [ -n "$artifact_file" ]; do
  case "$artifact_file" in
    ""|/*|*\\* )
      echo "ERROR: unsafe page file in mp-weixin manifest: $artifact_file" >&2
      exit 1
      ;;
    *.js|*.json|*.wxml|*.wxss ) ;;
    * )
      echo "ERROR: unexpected page file in mp-weixin manifest: $artifact_file" >&2
      exit 1
      ;;
  esac
  case "/$artifact_file/" in
    *"/../"* )
      echo "ERROR: unsafe page file in mp-weixin manifest: $artifact_file" >&2
      exit 1
      ;;
  esac
  if [ ! -f "$ARTIFACT_DIR/$artifact_file" ]; then
    echo "ERROR: mp-weixin artifact is missing declared page file: $artifact_file" >&2
    exit 1
  fi
  file_count=$((file_count + 1))
done < "$MANIFEST"

if [ "$file_count" -eq 0 ]; then
  echo "ERROR: mp-weixin page manifest is empty" >&2
  exit 1
fi

echo "Verified $file_count files from PAGE_FILE_MANIFEST."
