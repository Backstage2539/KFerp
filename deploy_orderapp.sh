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
COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata --exclude='._*' --exclude='*/._*' --exclude='./frontend-vue-shell/node_modules' --exclude='./frontend-vue-shell/.vite' -C orderapp-remote -cf - . | ssh -i "$KEY" "$SERVER" "tar -C $APP_DIR -xf -"

# 3) Sync build-time evidence context.
# Keep orderapp-remote/docs as the deployed app docs; those files are the
# Vue/API manual copies that support tests validate. Only add root acceptance
# evidence and miniapp source so Docker build tests can read sibling repo paths.
ssh -i "$KEY" "$SERVER" "mkdir -p $DOCS_DIR"
shopt -s nullglob
DOC_FILES=(REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md OPERATION_MANUALS.md OP_MANUAL_*.md DEPLOYMENT.md)
ssh -i "$KEY" "$SERVER" "mkdir -p $DOCS_DIR/workspace"
scp -i "$KEY" "${DOC_FILES[@]}" "$SERVER:$DOCS_DIR/workspace/"
if [ -d docs/acceptance ]; then
  COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata --exclude='._*' --exclude='*/._*' -C docs -cf - acceptance | ssh -i "$KEY" "$SERVER" "tar -C $DOCS_DIR -xf -"
fi
if [ -d miniapp ]; then
  ssh -i "$KEY" "$SERVER" "rm -rf $APP_DIR/miniapp && mkdir -p $APP_DIR/miniapp"
  COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata --exclude='._*' --exclude='*/._*' --exclude='./node_modules' --exclude='./dist' -C miniapp -cf - . | ssh -i "$KEY" "$SERVER" "tar -C $APP_DIR/miniapp -xf -"
fi
ssh -i "$KEY" "$SERVER" "set -e; mkdir -p /opt/stacks/erp/orderapp_data/shipping_exports; if [ -f /data/ship_temp.xlsx ]; then cp /data/ship_temp.xlsx /opt/stacks/erp/orderapp_data/ship_temp.xlsx; fi"

# 4) Ensure the DOCX conversion service is isolated from the app image.
ssh -i "$KEY" "$SERVER" "cat > /opt/stacks/erp/docker-compose.docconvert.yml <<'YAML'
services:
  orderapp:
    depends_on:
      docconvert:
        condition: service_started
    environment:
      DOCX_CONVERTER_URL: http://docconvert:3000/forms/libreoffice/convert

  docconvert:
    image: \${DOCX_CONVERTER_IMAGE:-docker.m.daocloud.io/gotenberg/gotenberg:8-libreoffice}
    container_name: erp_docconvert
    restart: unless-stopped
YAML"

# 5) Build & restart
ssh -i "$KEY" "$SERVER" "cd /opt/stacks/erp && docker compose -f docker-compose.yml -f docker-compose.docconvert.yml pull docconvert && docker compose -f docker-compose.yml -f docker-compose.docconvert.yml up -d docconvert && docker compose -f docker-compose.yml -f docker-compose.docconvert.yml build orderapp && docker compose -f docker-compose.yml -f docker-compose.docconvert.yml up -d orderapp"

echo "Deployed origin/develop=$REMOTE_HEAD with docs synced to $SERVER:$DOCS_DIR"
echo "Previous app backup: $SERVER:$BACKUP"
