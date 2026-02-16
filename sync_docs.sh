#!/usr/bin/env bash
set -euo pipefail

# Sync docs to server project directory
# Usage: ./sync_docs.sh

SERVER="root@1.12.242.58"
DEST_DIR="/opt/stacks/erp/orderapp/docs"
KEY="openclaw_jj_ed25519"

mkdir -p .
ssh -i "$KEY" "$SERVER" "mkdir -p $DEST_DIR"
scp -i "$KEY" REQUIREMENTS.md ACCEPTANCE_TESTS.md "$SERVER:$DEST_DIR/"
echo "Synced to $SERVER:$DEST_DIR"
