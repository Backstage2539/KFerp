#!/usr/bin/env bash
set -euo pipefail

# Deploy orderapp code + docs to server, rebuild and restart containers.
# Usage:
#   ./deploy_orderapp.sh              # branch default: main=production, develop=development
#   ./deploy_orderapp.sh production
#   ./deploy_orderapp.sh development

KEY="openclaw_jj_ed25519"
SERVER="root@1.12.242.58"
TARGET_ENV="${1:-}"

case "$TARGET_ENV" in
  "" )
    ;;
  production|development )
    ;;
  -h|--help )
    sed -n '1,12p' "$0"
    exit 0
    ;;
  * )
    echo "ERROR: expected target environment production|development, got $TARGET_ENV" >&2
    exit 1
    ;;
esac

# 0) Ensure local branch has been pushed to the matching release branch.
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ -z "$TARGET_ENV" ]; then
  case "$BRANCH" in
    main )
      TARGET_ENV="production"
      ;;
    develop )
      TARGET_ENV="development"
      ;;
    * )
      echo "ERROR: target environment is required on branch $BRANCH" >&2
      exit 1
      ;;
  esac
fi

case "$TARGET_ENV" in
  production )
    REQUIRED_BRANCH="main"
    STACK_DIR="/opt/stacks/erp-production"
    DOC_CONVERT_CONTAINER="erp_prod_docconvert"
    PUBLIC_URL="${PRODUCTION_PUBLIC_URL:-https://erp.qacoohee.com/app/}"
    ;;
  development )
    REQUIRED_BRANCH="develop"
    STACK_DIR="/opt/stacks/erp"
    DOC_CONVERT_CONTAINER="erp_docconvert"
    PUBLIC_URL="${DEVELOPMENT_PUBLIC_URL:-https://dev.erp.qacoohee.com/app/}"
    ;;
esac

APP_DIR="$STACK_DIR/orderapp"
DOCS_DIR="$APP_DIR/docs"

if [ "$BRANCH" != "$REQUIRED_BRANCH" ]; then
  echo "ERROR: $TARGET_ENV deploy requires branch=$REQUIRED_BRANCH, got $BRANCH" >&2
  exit 1
fi
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  echo "ERROR: tracked working tree not clean; commit first" >&2
  exit 1
fi
if git remote get-url origin >/dev/null 2>&1; then
  git fetch origin "$REQUIRED_BRANCH" >/dev/null 2>&1 || true
  LOCAL_HEAD="$(git rev-parse HEAD)"
  REMOTE_HEAD="$(git rev-parse "origin/$REQUIRED_BRANCH" 2>/dev/null || echo '')"
  if [ "$REMOTE_HEAD" = "" ]; then
    echo "ERROR: origin/$REQUIRED_BRANCH not found; push first" >&2
    exit 1
  fi
  if [ "$LOCAL_HEAD" != "$REMOTE_HEAD" ]; then
    echo "ERROR: local HEAD not pushed to origin/$REQUIRED_BRANCH; push first" >&2
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

if [ -d miniapp ]; then
  echo "Building miniapp mp-weixin..."
  cd miniapp
  npm ci
  npm run typecheck
  npm run build:mp-weixin
  test -d dist/build/mp-weixin
  cd ..
fi

# 2) Replace app source so deleted files do not linger on the server.
BACKUP="$APP_DIR.backup.deploy-$(date +%Y%m%d%H%M%S)"
ssh -i "$KEY" "$SERVER" "set -e; mkdir -p $STACK_DIR; cd $STACK_DIR; if [ -d orderapp ]; then mv orderapp $BACKUP; fi; mkdir -p orderapp"
COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata --exclude='._*' --exclude='*/._*' --exclude='./frontend-vue-shell/node_modules' --exclude='./frontend-vue-shell/.vite' -C orderapp-remote -cf - . | ssh -i "$KEY" "$SERVER" "tar -C $APP_DIR -xf -"

# 3) Sync build-time evidence context.
# Keep orderapp-remote/docs as the deployed app docs; those files are the
# single source for Vue/API operation manuals. Only add root governance docs,
# acceptance evidence and miniapp source so Docker build tests can read sibling
# repo paths.
ssh -i "$KEY" "$SERVER" "mkdir -p $DOCS_DIR"
shopt -s nullglob
DOC_FILES=(REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md DEPLOYMENT.md)
ssh -i "$KEY" "$SERVER" "mkdir -p $DOCS_DIR/workspace"
scp -i "$KEY" "${DOC_FILES[@]}" "$SERVER:$DOCS_DIR/workspace/"
if [ -d docs/acceptance ]; then
  COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata --exclude='._*' --exclude='*/._*' -C docs -cf - acceptance | ssh -i "$KEY" "$SERVER" "tar -C $DOCS_DIR -xf -"
fi
if [ -d miniapp ]; then
  ssh -i "$KEY" "$SERVER" "rm -rf $APP_DIR/miniapp && mkdir -p $APP_DIR/miniapp"
  COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata --exclude='._*' --exclude='*/._*' --exclude='./node_modules' -C miniapp -cf - . | ssh -i "$KEY" "$SERVER" "tar -C $APP_DIR/miniapp -xf - && test -d $APP_DIR/miniapp/dist/build/mp-weixin"
fi
ssh -i "$KEY" "$SERVER" "set -e; mkdir -p /opt/stacks/erp/orderapp_data/shipping_exports; if [ -f /data/ship_temp.xlsx ]; then cp /data/ship_temp.xlsx /opt/stacks/erp/orderapp_data/ship_temp.xlsx; fi"

# 4) Ensure the DOCX conversion service is isolated from the app image.
ssh -i "$KEY" "$SERVER" "cat > $STACK_DIR/docker-compose.docconvert.yml <<'YAML'
services:
  orderapp:
    depends_on:
      docconvert:
        condition: service_started
    environment:
      DOCX_CONVERTER_URL: http://docconvert:3000/forms/libreoffice/convert
      WECHAT_MINI_APP_ID: \${WECHAT_MINI_APP_ID:-}
      WECHAT_MINI_APP_SECRET: \${WECHAT_MINI_APP_SECRET:-}

  docconvert:
    image: \${DOCX_CONVERTER_IMAGE:-docker.m.daocloud.io/gotenberg/gotenberg:8-libreoffice}
    container_name: ${DOC_CONVERT_CONTAINER}
    restart: unless-stopped
YAML"

# 5) Build & restart
ssh -i "$KEY" "$SERVER" "cd $STACK_DIR && docker compose -f docker-compose.yml -f docker-compose.docconvert.yml pull docconvert && docker compose -f docker-compose.yml -f docker-compose.docconvert.yml up -d docconvert && docker compose -f docker-compose.yml -f docker-compose.docconvert.yml build orderapp && docker compose -f docker-compose.yml -f docker-compose.docconvert.yml up -d orderapp"

echo "Deployed $TARGET_ENV origin/$REQUIRED_BRANCH=$REMOTE_HEAD with docs synced to $SERVER:$DOCS_DIR"
echo "Frontend URL: $PUBLIC_URL"
echo "Previous app backup: $SERVER:$BACKUP"
