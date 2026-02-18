#!/usr/bin/env bash
set -euo pipefail

# Deploy orderapp code + docs to server, rebuild and restart containers.
# Usage: ./deploy_orderapp.sh

KEY="openclaw_jj_ed25519"
SERVER="root@1.12.242.58"
APP_DIR="/opt/stacks/erp/orderapp"
DOCS_DIR="$APP_DIR/docs"

# 0) Ensure local code has been pushed (C1)
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$BRANCH" != "main" ]; then
  echo "ERROR: deploy requires branch=main, got $BRANCH" >&2
  exit 1
fi
if [ -n "$(git status --porcelain)" ]; then
  echo "ERROR: working tree not clean; commit first" >&2
  exit 1
fi
if git remote get-url origin >/dev/null 2>&1; then
  git fetch origin main >/dev/null 2>&1 || true
  LOCAL_HEAD="$(git rev-parse HEAD)"
  REMOTE_HEAD="$(git rev-parse origin/main 2>/dev/null || echo '')"
  if [ "$REMOTE_HEAD" = "" ]; then
    echo "ERROR: origin/main not found; push first" >&2
    exit 1
  fi
  if [ "$LOCAL_HEAD" != "$REMOTE_HEAD" ]; then
    echo "ERROR: local HEAD not pushed to origin/main; push first" >&2
    echo "  local:  $LOCAL_HEAD" >&2
    echo "  origin: $REMOTE_HEAD" >&2
    exit 1
  fi
else
  echo "ERROR: no git remote origin; cannot verify push" >&2
  exit 1
fi

# 1) Sync docs
ssh -i "$KEY" "$SERVER" "mkdir -p $DOCS_DIR"
scp -i "$KEY" REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md DEPLOYMENT.md "$SERVER:$DOCS_DIR/"

# 2) Build frontend (React)
echo "Building frontend..."
cd orderapp-remote/frontend
npm ci 2>/dev/null || npm install
npm run build
cd ../..

# 3) Sync app source (adjust list as needed)
# Note: we copy the whole orderapp-remote folder to keep it simple.
scp -i "$KEY" -r orderapp-remote/* "$SERVER:$APP_DIR/"

# 3) Build & restart
ssh -i "$KEY" "$SERVER" "cd /opt/stacks/erp && docker compose build orderapp && docker compose up -d"

echo "Deployed with docs synced to $SERVER:$DOCS_DIR"
