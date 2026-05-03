#!/usr/bin/env bash
set -euo pipefail

# Deploy orderapp code + docs to a named environment, rebuild and restart containers.
# Usage:
#   ./deploy_orderapp.sh                  # production (formal online release)
#   ./deploy_orderapp.sh production
#   ./deploy_orderapp.sh development      # legacy develop environment
#   ./deploy_orderapp.sh --print-plan [production|development]
#   ./deploy_orderapp.sh --switch-public production|development

KEY="openclaw_jj_ed25519"
SERVER="root@1.12.242.58"
DEPLOY_ENV="${DEPLOY_ENV:-}"
MODE="deploy"

if [ "${1:-}" = "--print-plan" ]; then
  MODE="print-plan"
  shift
fi
if [ "${1:-}" = "--switch-public" ]; then
  MODE="switch-public"
  shift
fi

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  sed -n '2,12p' "$0"
  exit 0
fi

if [ $# -gt 1 ]; then
  echo "ERROR: expected at most one environment argument" >&2
  exit 1
fi

if [ $# -eq 1 ]; then
  if [ -n "$DEPLOY_ENV" ] && [ "$DEPLOY_ENV" != "$1" ]; then
    echo "ERROR: DEPLOY_ENV=$DEPLOY_ENV conflicts with argument $1" >&2
    exit 1
  fi
  DEPLOY_ENV="$1"
fi

DEPLOY_ENV="${DEPLOY_ENV:-production}"

case "$DEPLOY_ENV" in
  prod|production)
    DEPLOY_ENV="production"
    REQUIRED_BRANCH="main"
    REMOTE_BRANCH="main"
    STACK_DIR="/opt/stacks/erp-production"
    OTHER_ENV="development"
    OTHER_STACK_DIR="/opt/stacks/erp"
    ;;
  dev|development)
    DEPLOY_ENV="development"
    REQUIRED_BRANCH="develop"
    REMOTE_BRANCH="develop"
    STACK_DIR="/opt/stacks/erp"
    OTHER_ENV="production"
    OTHER_STACK_DIR="/opt/stacks/erp-production"
    ;;
  *)
    echo "ERROR: unknown deploy environment '$DEPLOY_ENV' (use production or development)" >&2
    exit 1
    ;;
esac

REMOTE_REF="origin/$REMOTE_BRANCH"
APP_DIR="$STACK_DIR/orderapp"
DOCS_DIR="$APP_DIR/docs"

if [ "$MODE" = "print-plan" ]; then
  cat <<PLAN
deploy_env=$DEPLOY_ENV
server=$SERVER
required_branch=$REQUIRED_BRANCH
remote_ref=$REMOTE_REF
stack_dir=$STACK_DIR
app_dir=$APP_DIR
docs_dir=$DOCS_DIR
compose_service=orderapp
other_env=$OTHER_ENV
other_stack_dir=$OTHER_STACK_DIR
PLAN
  exit 0
fi

SSH_KEY_ARGS=(-o BatchMode=yes)
if [ -f "$KEY" ]; then
  SSH_KEY_ARGS+=(-i "$KEY")
fi

if [ "$MODE" = "switch-public" ]; then
  echo "Switching public endpoint to $DEPLOY_ENV"
  echo "Target stack: $SERVER:$STACK_DIR"
  echo "Stopping Caddy in $OTHER_ENV stack only: $OTHER_STACK_DIR"
  echo "Starting Caddy in $DEPLOY_ENV stack only: $STACK_DIR"
  ssh "${SSH_KEY_ARGS[@]}" "$SERVER" "set -e; test -f $STACK_DIR/docker-compose.yml || { echo 'ERROR: missing $STACK_DIR/docker-compose.yml' >&2; exit 1; }; test -f $OTHER_STACK_DIR/docker-compose.yml || { echo 'ERROR: missing $OTHER_STACK_DIR/docker-compose.yml' >&2; exit 1; }; cd $OTHER_STACK_DIR; docker compose stop caddy || true; cd $STACK_DIR; docker compose up -d caddy; echo 'target stack:'; docker compose ps; echo 'other stack:'; cd $OTHER_STACK_DIR; docker compose ps"
  echo "Public endpoint switched to $DEPLOY_ENV. Verify with curl/browser before handing off."
  exit 0
fi

echo "Deploy environment: $DEPLOY_ENV"
echo "Target branch: $REQUIRED_BRANCH ($REMOTE_REF)"
echo "Target stack: $SERVER:$STACK_DIR"

# 0) Ensure local branch has been pushed to its environment branch.
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$BRANCH" != "$REQUIRED_BRANCH" ]; then
  echo "ERROR: $DEPLOY_ENV deploy requires branch=$REQUIRED_BRANCH, got $BRANCH" >&2
  exit 1
fi
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  echo "ERROR: tracked working tree not clean; commit first" >&2
  exit 1
fi
if git remote get-url origin >/dev/null 2>&1; then
  git fetch origin "$REMOTE_BRANCH" >/dev/null 2>&1 || true
  LOCAL_HEAD="$(git rev-parse HEAD)"
  REMOTE_HEAD="$(git rev-parse "$REMOTE_REF" 2>/dev/null || echo '')"
  if [ "$REMOTE_HEAD" = "" ]; then
    echo "ERROR: $REMOTE_REF not found; push first" >&2
    exit 1
  fi
  if [ "$LOCAL_HEAD" != "$REMOTE_HEAD" ]; then
    echo "ERROR: local HEAD not pushed to $REMOTE_REF; push first" >&2
    echo "  local:  $LOCAL_HEAD" >&2
    echo "  $REMOTE_REF: $REMOTE_HEAD" >&2
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

# 2) Replace app source so deleted files do not linger on the target environment.
BACKUP="$APP_DIR.backup.deploy-$(date +%Y%m%d%H%M%S)"
ssh "${SSH_KEY_ARGS[@]}" "$SERVER" "test -f $STACK_DIR/docker-compose.yml || { echo 'ERROR: missing $STACK_DIR/docker-compose.yml for $DEPLOY_ENV environment' >&2; exit 1; }"
ssh "${SSH_KEY_ARGS[@]}" "$SERVER" "set -e; cd $STACK_DIR; if [ -d orderapp ]; then mv orderapp $BACKUP; fi; mkdir -p orderapp"
COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata --exclude='._*' --exclude='*/._*' --exclude='./frontend-vue-shell/node_modules' --exclude='./frontend-vue-shell/.vite' -C orderapp-remote -cf - . | ssh "${SSH_KEY_ARGS[@]}" "$SERVER" "tar -C $APP_DIR -xf -"

# 3) Sync docs
ssh "${SSH_KEY_ARGS[@]}" "$SERVER" "mkdir -p $DOCS_DIR"
scp "${SSH_KEY_ARGS[@]}" REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md DEPLOYMENT.md "$SERVER:$DOCS_DIR/"
ssh "${SSH_KEY_ARGS[@]}" "$SERVER" "set -e; mkdir -p $STACK_DIR/orderapp_data/shipping_exports; if [ -f /data/ship_temp.xlsx ]; then cp /data/ship_temp.xlsx $STACK_DIR/orderapp_data/ship_temp.xlsx; fi"

# 4) Build & restart
ssh "${SSH_KEY_ARGS[@]}" "$SERVER" "cd $STACK_DIR && docker compose build orderapp && docker compose up -d orderapp"

echo "Deployed $DEPLOY_ENV $REMOTE_REF=$REMOTE_HEAD with docs synced to $SERVER:$DOCS_DIR"
echo "Previous app backup: $SERVER:$BACKUP"
