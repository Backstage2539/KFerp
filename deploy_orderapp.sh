#!/usr/bin/env bash
set -euo pipefail

# Deploy orderapp code + docs to server, rebuild and restart containers.
# Usage: ./deploy_orderapp.sh

KEY="openclaw_jj_ed25519"
SERVER="root@1.12.242.58"
APP_DIR="/opt/stacks/erp/orderapp"
DOCS_DIR="$APP_DIR/docs"

# 0) Ensure local develop has been pushed
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$BRANCH" != "develop" ]; then
  echo "ERROR: deploy requires branch=develop, got $BRANCH" >&2
  exit 1
fi
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  echo "ERROR: tracked working tree not clean; commit first" >&2
  exit 1
fi
if git remote get-url origin >/dev/null 2>&1; then
  git fetch origin develop >/dev/null 2>&1 || true
  LOCAL_HEAD="$(git rev-parse HEAD)"
  REMOTE_HEAD="$(git rev-parse origin/develop 2>/dev/null || echo '')"
  if [ "$REMOTE_HEAD" = "" ]; then
    echo "ERROR: origin/develop not found; push first" >&2
    exit 1
  fi
  if [ "$LOCAL_HEAD" != "$REMOTE_HEAD" ]; then
    echo "ERROR: local HEAD not pushed to origin/develop; push first" >&2
    echo "  local:  $LOCAL_HEAD" >&2
    echo "  origin: $REMOTE_HEAD" >&2
    exit 1
  fi
else
  echo "ERROR: no git remote origin; cannot verify push" >&2
  exit 1
fi

# 1) Build frontend (Vue shell)
echo "Building Vue shell..."
cd orderapp-remote/frontend-vue-shell
npm ci 2>/dev/null || npm install
npm run build
cd ../..

# 2) Replace app source so deleted files do not linger on the server.
BACKUP="$APP_DIR.backup.deploy-$(date +%Y%m%d%H%M%S)"
ssh -i "$KEY" "$SERVER" "set -e; cd /opt/stacks/erp; if [ -d orderapp ]; then mv orderapp $BACKUP; fi; mkdir -p orderapp"
tar --exclude='./frontend-vue-shell/node_modules' --exclude='./frontend-vue-shell/.vite' -C orderapp-remote -cf - . | ssh -i "$KEY" "$SERVER" "tar -C $APP_DIR -xf -"

# 3) Sync docs
ssh -i "$KEY" "$SERVER" "mkdir -p $DOCS_DIR"
scp -i "$KEY" REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md DEPLOYMENT.md "$SERVER:$DOCS_DIR/"

# 4) Build & restart
ssh -i "$KEY" "$SERVER" "cd /opt/stacks/erp && docker compose build orderapp && docker compose up -d orderapp"

echo "Deployed origin/develop=$REMOTE_HEAD with docs synced to $SERVER:$DOCS_DIR"
echo "Previous app backup: $SERVER:$BACKUP"
