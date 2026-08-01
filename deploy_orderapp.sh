#!/usr/bin/env bash
set -euo pipefail

# KFerp remote build + deployment entrypoint.
#
# The Mac only validates git state and streams the committed source archive.
# npm, Vue/UniApp builds, Go tests and Docker builds run serially on the server.
#
# Usage:
#   ./deploy_orderapp.sh                 # main=production, develop=development
#   ./deploy_orderapp.sh production
#   ./deploy_orderapp.sh development
#   ./deploy_orderapp.sh --preflight development  # pushed feature branch, no restart
#
# Optional environment variables:
#   KFERP_SSH_KEY=/absolute/key          # otherwise use SSH agent/default keys
#   KFERP_MINIAPP_EXPORT_DIR=/path       # production artifact destination
#   KFERP_MINIAPP_DEVELOPMENT_EXPORT_DIR=/path  # development artifact destination
#   KFERP_SKIP_MINIAPP_EXPORT=1          # do not pull the target mp-weixin artifact

# Remote helper contract and source-level regression markers. The commands run
# in scripts/remote_orderapp_release.sh, never on the Mac:
#   TARGET_ENV="production" / TARGET_ENV="development"
#   frontend-vue-shell: npm ci, node --test, npm run build
#   miniapp: npm ci, npm run typecheck, npm run build:mp-weixin
#   docker compose -f docker-compose.yml -f docker-compose.docconvert.yml build orderapp
#   WECHAT_MINI_APP_ID: \${WECHAT_MINI_APP_ID:-}
#   WECHAT_MINI_APP_SECRET: \${WECHAT_MINI_APP_SECRET:-}
#   DOCX_CONVERTER_URL: http://docconvert:3000/forms/libreoffice/convert
#   docker.m.daocloud.io/gotenberg/gotenberg:8-libreoffice
#   mkdir -p $STACK_DIR/orderapp_data/shipping_exports
#   cp /data/ship_temp.xlsx $STACK_DIR/orderapp_data/ship_temp.xlsx
#   DOC_CONVERT_STATE=; docker inspect --format; running|healthy|starting
#   up -d --pull missing --force-recreate docconvert
# Keep orderapp-remote/docs as the deployed app docs; root governance files go
# into its workspace context before the remote Docker build.

SERVER="${KFERP_DEPLOY_SERVER:-root@1.12.242.58}"
PREFLIGHT=0
if [ "${1:-}" = "--preflight" ]; then
  PREFLIGHT=1
  shift
fi
TARGET_ENV="${1:-}"
if [ "$#" -gt 1 ]; then
  echo "ERROR: too many arguments" >&2
  exit 1
fi

case "$TARGET_ENV" in
  ""|production|development ) ;;
  -h|--help ) sed -n '1,28p' "$0"; exit 0 ;;
  * ) echo "ERROR: expected target environment production|development, got $TARGET_ENV" >&2; exit 1 ;;
esac

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ -z "$TARGET_ENV" ]; then
  if [ "$PREFLIGHT" -eq 1 ]; then
    echo "ERROR: --preflight requires development or production" >&2
    exit 1
  fi
  case "$BRANCH" in
    main ) TARGET_ENV="production" ;;
    develop ) TARGET_ENV="development" ;;
    * ) echo "ERROR: target environment is required on branch $BRANCH" >&2; exit 1 ;;
  esac
fi

case "$TARGET_ENV" in
  production )
    REQUIRED_BRANCH="main"
    STACK_DIR="/opt/stacks/erp-production"
    ORDERAPP_CONTAINER="erp_prod_orderapp"
    DOC_CONVERT_CONTAINER="erp_prod_docconvert"
    API_BASE="https://erp.qacoohee.com/app"
    PUBLIC_URL="${PRODUCTION_PUBLIC_URL:-https://erp.qacoohee.com/app/}"
    ;;
  development )
    REQUIRED_BRANCH="develop"
    STACK_DIR="/opt/stacks/erp"
    ORDERAPP_CONTAINER="erp_orderapp"
    DOC_CONVERT_CONTAINER="erp_docconvert"
    API_BASE="https://dev.qacoohee.com/app"
    PUBLIC_URL="${DEVELOPMENT_PUBLIC_URL:-https://dev.qacoohee.com/app/}"
    ;;
esac
APP_DIR="$STACK_DIR/orderapp"

if [ "$PREFLIGHT" -eq 0 ] && [ "$BRANCH" != "$REQUIRED_BRANCH" ]; then
  echo "ERROR: $TARGET_ENV deploy requires branch=$REQUIRED_BRANCH, got $BRANCH" >&2
  exit 1
fi
if [ -n "$(git status --porcelain)" ]; then
  echo "ERROR: working tree not clean; commit or remove untracked files first" >&2
  exit 1
fi
if ! git remote get-url origin >/dev/null 2>&1; then
  echo "ERROR: no git remote origin; cannot verify pushed release commit" >&2
  exit 1
fi

LOCAL_HEAD="$(git rev-parse HEAD)"
if [ "$PREFLIGHT" -eq 1 ]; then
  UPSTREAM="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)"
  case "$UPSTREAM" in
    origin/* ) ;;
    * ) echo "ERROR: preflight requires a pushed branch with an origin upstream" >&2; exit 1 ;;
  esac
  git fetch --quiet origin
  REMOTE_HEAD="$(git rev-parse "$UPSTREAM")"
  if [ "$LOCAL_HEAD" != "$REMOTE_HEAD" ]; then
    echo "ERROR: local HEAD is not the pushed upstream commit; push the feature branch first" >&2
    echo "  local:    $LOCAL_HEAD" >&2
    echo "  upstream: $REMOTE_HEAD" >&2
    exit 1
  fi
else
  git fetch --quiet origin "$REQUIRED_BRANCH"
  REMOTE_HEAD="$(git rev-parse "origin/$REQUIRED_BRANCH")"
  if [ "$LOCAL_HEAD" != "$REMOTE_HEAD" ]; then
    echo "ERROR: local HEAD is not the current origin/$REQUIRED_BRANCH; push or update first" >&2
    echo "  local:  $LOCAL_HEAD" >&2
    echo "  origin: $REMOTE_HEAD" >&2
    exit 1
  fi
fi

SSH_ARGS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new)
if [ -n "${KFERP_SSH_KEY:-}" ]; then
  if [ ! -f "$KFERP_SSH_KEY" ]; then
    echo "ERROR: KFERP_SSH_KEY does not exist: $KFERP_SSH_KEY" >&2
    exit 1
  fi
  SSH_ARGS+=(-i "$KFERP_SSH_KEY")
elif [ -f "$REPO_ROOT/openclaw_jj_ed25519" ]; then
  SSH_ARGS+=(-i "$REPO_ROOT/openclaw_jj_ed25519")
fi

REMOTE_STAGE="/tmp/kferp-orderapp-release-$TARGET_ENV-${REMOTE_HEAD:0:12}-$(date +%Y%m%d%H%M%S)-$$"
REMOTE_STAGE_CREATED=0

cleanup_remote_stage() {
  local status=$?
  set +e
  if [ "$REMOTE_STAGE_CREATED" -eq 1 ]; then
    case "$REMOTE_STAGE" in
      /tmp/kferp-orderapp-release-* )
        printf -v cleanup_command 'rm -rf %q' "$REMOTE_STAGE"
        ssh "${SSH_ARGS[@]}" "$SERVER" "$cleanup_command" >/dev/null 2>&1
        ;;
    esac
  fi
  return "$status"
}
trap cleanup_remote_stage EXIT

echo "Preparing committed source $REMOTE_HEAD for remote $TARGET_ENV build..."
printf -v prepare_command 'mkdir -p %q' "$REMOTE_STAGE/repo"
ssh "${SSH_ARGS[@]}" "$SERVER" "$prepare_command"
REMOTE_STAGE_CREATED=1

# git archive guarantees that the server receives exactly the clean, pushed
# commit. No node_modules, local build output, caches or untracked secrets are
# read from or uploaded by this step.
git archive --format=tar HEAD -- \
  .agents \
  orderapp-remote \
  miniapp \
  scripts \
  docs/acceptance \
  REQUIREMENTS.md \
  ACCEPTANCE_TESTS.md \
  ACTIVE_REQUIREMENTS.md \
  HOW_TO_WORKFLOW.md \
  DEPLOYMENT.md \
  deploy_orderapp.sh \
  | ssh "${SSH_ARGS[@]}" "$SERVER" "tar -C '$REMOTE_STAGE/repo' -xf -"

printf -v marker_command 'printf %%s %q > %q' "$REMOTE_HEAD" "$REMOTE_STAGE/repo/.release-commit"
ssh "${SSH_ARGS[@]}" "$SERVER" "$marker_command"

remote_release_args=(
  bash "$REMOTE_STAGE/repo/scripts/remote_orderapp_release.sh"
  --environment "$TARGET_ENV"
  --stack-dir "$STACK_DIR"
  --source-root "$REMOTE_STAGE/repo"
  --expected-commit "$REMOTE_HEAD"
  --api-base "$API_BASE"
  --orderapp-container "$ORDERAPP_CONTAINER"
  --docconvert-container "$DOC_CONVERT_CONTAINER"
)
if [ "$PREFLIGHT" -eq 1 ]; then
  remote_release_args+=(--preflight)
fi
printf -v remote_release_command '%q ' "${remote_release_args[@]}"

if [ "$PREFLIGHT" -eq 1 ]; then
  echo "Starting locked, single-worker server preflight. No stack files or containers will be changed."
else
  echo "Starting locked, single-worker server build. This rebuilds and restarts only $ORDERAPP_CONTAINER."
fi
ssh "${SSH_ARGS[@]}" "$SERVER" "$remote_release_command"
REMOTE_STAGE_CREATED=0

external_smoke() {
  local smoke_url="${PUBLIC_URL%/}/login"
  local smoke_code=""
  local curl_args=(-sS --connect-timeout 3 --max-time 8)

  if ! command -v curl >/dev/null 2>&1; then
    echo "ERROR: curl is required for the external deployment smoke" >&2
    return 1
  fi
  if [ "$TARGET_ENV" = "development" ]; then
    # Force the known public ingress IP so a local proxy or fake-IP DNS mapping
    # cannot send the smoke elsewhere. Keep certificate validation enabled.
    curl_args+=(
      --resolve "dev.qacoohee.com:443:${KFERP_DEVELOPMENT_PUBLIC_IP:-1.12.242.58}"
    )
  fi

  for _external_attempt in $(seq 1 15); do
    smoke_code="$(curl "${curl_args[@]}" -o /dev/null -w '%{http_code}' "$smoke_url" 2>/dev/null || true)"
    case "$smoke_code" in
      200|301|302|303|401 )
        echo "External smoke passed: $smoke_url returned HTTP $smoke_code"
        return 0
        ;;
    esac
    sleep 2
  done
  echo "ERROR: server deployment is internally healthy, but external smoke failed: $smoke_url returned HTTP ${smoke_code:-none}" >&2
  echo "The deployed server release was not rolled back for a client-network or ingress-only failure; stop before promoting the next environment." >&2
  return 1
}

sync_miniapp_artifact() {
  local target_dir=""
  case "$TARGET_ENV" in
    development ) target_dir="${KFERP_MINIAPP_DEVELOPMENT_EXPORT_DIR:-/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev}" ;;
    production ) target_dir="${KFERP_MINIAPP_EXPORT_DIR:-/Users/yiiiple-work/KFerp-miniapp-mp-weixin}" ;;
  esac
  local target_parent
  local incoming_root
  local incoming_dir
  local backup_dir=""

  case "$TARGET_ENV:$target_dir" in
    development:/*/KFerp-miniapp-mp-weixin-dev ) ;;
    production:/*/KFerp-miniapp-mp-weixin ) ;;
    * ) echo "ERROR: refusing unexpected miniapp export target: $target_dir" >&2; return 1 ;;
  esac
  target_parent="$(dirname "$target_dir")"
  if [ ! -d "$target_parent" ]; then
    echo "ERROR: miniapp export parent does not exist: $target_parent" >&2
    return 1
  fi

  incoming_root="$(mktemp -d "$target_parent/.kferp-miniapp-sync.XXXXXX")"
  incoming_dir="$incoming_root/incoming"
  mkdir "$incoming_dir"
  printf -v artifact_check 'test -f %q && test -f %q && test -f %q && test -f %q && test -d %q && tar -C %q -cf - .' \
    "$APP_DIR/miniapp/dist/build/mp-weixin/app.json" \
    "$APP_DIR/miniapp/dist/build/mp-weixin/project.config.json" \
    "$APP_DIR/miniapp/dist/build/mp-weixin/RELEASE_INFO" \
    "$APP_DIR/miniapp/dist/build/mp-weixin/PAGE_FILE_MANIFEST" \
    "$APP_DIR/miniapp/dist/build/mp-weixin/pages" \
    "$APP_DIR/miniapp/dist/build/mp-weixin"

  if ! ssh "${SSH_ARGS[@]}" "$SERVER" "$artifact_check" | tar -C "$incoming_dir" -xf -; then
    rm -rf "$incoming_root"
    echo "ERROR: could not download the $TARGET_ENV mp-weixin artifact" >&2
    return 1
  fi
  if [ ! -f "$incoming_dir/app.json" ] || \
     [ ! -f "$incoming_dir/project.config.json" ] || \
     [ ! -d "$incoming_dir/pages" ] || \
     [ ! -f "$incoming_dir/RELEASE_INFO" ] || \
     [ ! -f "$incoming_dir/PAGE_FILE_MANIFEST" ]; then
    rm -rf "$incoming_root"
    echo "ERROR: downloaded mp-weixin artifact is incomplete" >&2
    return 1
  fi
  if ! bash "$REPO_ROOT/scripts/verify_mp_weixin_manifest.sh" "$incoming_dir"; then
    rm -rf "$incoming_root"
    echo "ERROR: downloaded mp-weixin artifact has an incomplete declared page" >&2
    return 1
  fi
  if ! grep -Fxq "commit=$REMOTE_HEAD" "$incoming_dir/RELEASE_INFO" || \
     ! grep -Fxq "environment=$TARGET_ENV" "$incoming_dir/RELEASE_INFO" || \
     ! grep -Fxq "api_base=$API_BASE" "$incoming_dir/RELEASE_INFO"; then
    rm -rf "$incoming_root"
    echo "ERROR: downloaded mp-weixin RELEASE_INFO does not match this production release" >&2
    return 1
  fi

  if [ -e "$target_dir" ]; then
    backup_dir="$target_dir.backup-$(date +%Y%m%d%H%M%S)-${REMOTE_HEAD:0:12}"
    if [ -e "$backup_dir" ]; then
      rm -rf "$incoming_root"
      echo "ERROR: miniapp backup destination already exists: $backup_dir" >&2
      return 1
    fi
  fi
  if ! bash "$REPO_ROOT/scripts/swap_miniapp_export.sh" "$target_dir" "$incoming_dir" "$backup_dir"; then
    rm -rf "$incoming_root"
    echo "ERROR: could not atomically replace $target_dir" >&2
    return 1
  fi
  rm -rf "$incoming_root"
  echo "$TARGET_ENV mp-weixin artifact synced to: $target_dir"
  if [ -n "$backup_dir" ]; then
    echo "Previous mp-weixin artifact retained at: $backup_dir"
  fi
  echo "If WeChat DevTools already has this fixed directory open, close and re-import $target_dir before preview or upload; clearing compile cache alone may retain a stale upload manifest."
}

if [ "$PREFLIGHT" -eq 0 ]; then
  external_smoke
fi

if [ "$PREFLIGHT" -eq 0 ] && [ "${KFERP_SKIP_MINIAPP_EXPORT:-0}" != "1" ]; then
  sync_miniapp_artifact
fi

if [ "$PREFLIGHT" -eq 1 ]; then
  echo "Remote preflight passed for $UPSTREAM=$REMOTE_HEAD using $TARGET_ENV build settings."
  echo "No server source, container, persistent stack file, or fixed miniapp directory was changed."
  exit 0
fi

echo "Deployed $TARGET_ENV origin/$REQUIRED_BRANCH=$REMOTE_HEAD"
echo "Frontend URL: $PUBLIC_URL"
echo "Server source: $SERVER:$APP_DIR"
if [ "$TARGET_ENV" = "production" ]; then
  echo "NOTICE: server deployment does not upload or publish a WeChat Mini Program version."
  echo "Import /Users/yiiiple-work/KFerp-miniapp-mp-weixin into WeChat DevTools, then upload/review/publish separately."
else
  echo "NOTICE: server deployment does not upload a WeChat Mini Program development or trial version."
  echo "Import /Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev into WeChat DevTools, then preview or upload separately."
fi
