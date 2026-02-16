#!/usr/bin/env bash
set -euo pipefail

# Deploy orderapp code + docs to server, rebuild and restart containers.
# Usage: ./deploy_orderapp.sh

KEY="openclaw_jj_ed25519"
SERVER="root@1.12.242.58"
APP_DIR="/opt/stacks/erp/orderapp"
DOCS_DIR="$APP_DIR/docs"

# 1) Sync docs
ssh -i "$KEY" "$SERVER" "mkdir -p $DOCS_DIR"
scp -i "$KEY" REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md "$SERVER:$DOCS_DIR/"

# 2) Sync app source (adjust list as needed)
# Note: we copy the whole orderapp-remote folder to keep it simple.
scp -i "$KEY" -r orderapp-remote/* "$SERVER:$APP_DIR/"

# 3) Build & restart
ssh -i "$KEY" "$SERVER" "cd /opt/stacks/erp && docker compose build orderapp && docker compose up -d"

echo "Deployed with docs synced to $SERVER:$DOCS_DIR"
