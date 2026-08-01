#!/usr/bin/env bash
set -euo pipefail

# Runs on the KFerp deployment server. The caller uploads a clean git archive
# to a unique /tmp directory, then this script serializes all resource-heavy
# checks, builds, source promotion and the orderapp restart under one lock.

usage() {
  cat <<'EOF'
Usage: remote_orderapp_release.sh \
  --environment production|development \
  --stack-dir /opt/stacks/... \
  --source-root /tmp/.../repo \
  --expected-commit <git-sha> \
  --api-base https://.../app \
  --orderapp-container <container> \
  --docconvert-container <container> \
  [--preflight]
EOF
}

TARGET_ENV=""
STACK_DIR=""
SOURCE_ROOT=""
EXPECTED_COMMIT=""
API_BASE=""
ORDERAPP_CONTAINER=""
DOC_CONVERT_CONTAINER=""
PREFLIGHT=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --environment ) TARGET_ENV="${2:-}"; shift 2 ;;
    --stack-dir ) STACK_DIR="${2:-}"; shift 2 ;;
    --source-root ) SOURCE_ROOT="${2:-}"; shift 2 ;;
    --expected-commit ) EXPECTED_COMMIT="${2:-}"; shift 2 ;;
    --api-base ) API_BASE="${2:-}"; shift 2 ;;
    --orderapp-container ) ORDERAPP_CONTAINER="${2:-}"; shift 2 ;;
    --docconvert-container ) DOC_CONVERT_CONTAINER="${2:-}"; shift 2 ;;
    --preflight ) PREFLIGHT=1; shift ;;
    -h|--help ) usage; exit 0 ;;
    * ) echo "ERROR: unknown argument $1" >&2; usage >&2; exit 1 ;;
  esac
done

case "$TARGET_ENV" in
  production|development ) ;;
  * ) echo "ERROR: invalid environment: $TARGET_ENV" >&2; exit 1 ;;
esac
case "$STACK_DIR" in
  /opt/stacks/erp|/opt/stacks/erp-production ) ;;
  * ) echo "ERROR: refusing unexpected stack directory: $STACK_DIR" >&2; exit 1 ;;
esac
case "$SOURCE_ROOT" in
  /tmp/kferp-orderapp-release-*/repo ) ;;
  * ) echo "ERROR: refusing unexpected temporary source root: $SOURCE_ROOT" >&2; exit 1 ;;
esac
if ! [[ "$EXPECTED_COMMIT" =~ ^[0-9a-f]{40}$ ]]; then
  echo "ERROR: expected commit must be a full git SHA" >&2
  exit 1
fi
if [ -z "$API_BASE" ] || [ -z "$ORDERAPP_CONTAINER" ] || [ -z "$DOC_CONVERT_CONTAINER" ]; then
  echo "ERROR: api base and container names are required" >&2
  exit 1
fi
case "$TARGET_ENV:$API_BASE" in
  development:https://dev.qacoohee.com/app ) ;;
  production:https://erp.qacoohee.com/app ) ;;
  * ) echo "ERROR: api base does not match environment: $API_BASE" >&2; exit 1 ;;
esac
if [ ! -f "$SOURCE_ROOT/.release-commit" ] || [ "$(cat "$SOURCE_ROOT/.release-commit")" != "$EXPECTED_COMMIT" ]; then
  echo "ERROR: uploaded source commit marker does not match $EXPECTED_COMMIT" >&2
  exit 1
fi

for command_name in curl flock docker go node npm tar; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "ERROR: required server command not found: $command_name" >&2
    exit 1
  fi
done

exec 9>/var/lock/kferp-orderapp-deploy.lock
if ! flock -n 9; then
  echo "ERROR: another KFerp build or deployment is running" >&2
  exit 1
fi

BUILD_ROOT="${SOURCE_ROOT%/repo}"
APP_DIR="$STACK_DIR/orderapp"
DOCS_DIR="$APP_DIR/docs"
VUE_DIR="$SOURCE_ROOT/orderapp-remote/frontend-vue-shell"
MINIAPP_DIR="$SOURCE_ROOT/miniapp"
RELEASE_DIR="$SOURCE_ROOT/orderapp-remote"
BUILD_OVERRIDE="$BUILD_ROOT/docker-compose.remote-release.yml"
DOC_CONVERT_OVERRIDE="$STACK_DIR/docker-compose.docconvert.yml"
DEPLOY_TS="$(date +%Y%m%d%H%M%S)"
SHORT_COMMIT="${EXPECTED_COMMIT:0:12}"
BACKUP="$APP_DIR.backup.deploy-$DEPLOY_TS-$SHORT_COMMIT"
FAILED_RELEASE="$APP_DIR.failed.deploy-$DEPLOY_TS-$SHORT_COMMIT"
ROLLBACK_IMAGE="kferp-orderapp-rollback:$TARGET_ENV-$DEPLOY_TS-$SHORT_COMMIT"
PREFLIGHT_IMAGE=""
OLD_IMAGE_ID=""
OLD_IMAGE_REF=""
PROMOTED=0
DEPLOY_OK=0
IMAGE_BUILT=0
PUBLIC_RESOLVE_ARGS=()
if [ "$TARGET_ENV" = "development" ]; then
  # Exercise the same public-certificate vhost through loopback; the caller
  # performs an additional strict external smoke against the public IP.
  PUBLIC_RESOLVE_ARGS=(--resolve dev.qacoohee.com:443:127.0.0.1)
fi

wait_for_orderapp_http() {
  local max_attempts="$1"
  local required_successes="$2"
  local consecutive_successes=0
  local running=""
  local health=""
  local response=""

  for _ready_attempt in $(seq 1 "$max_attempts"); do
    running="$(docker inspect --format '{{.State.Running}}' "$ORDERAPP_CONTAINER" 2>/dev/null || true)"
    health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$ORDERAPP_CONTAINER" 2>/dev/null || true)"
    if [ "$running" = "true" ] && [ "$health" != "unhealthy" ]; then
      response="$(docker exec "$ORDERAPP_CONTAINER" sh -ec \
        'wget -S -O /dev/null http://127.0.0.1:8080/login 2>&1 || true')"
      if printf '%s\n' "$response" | grep -Eq 'HTTP/[0-9.]+ (200|302|401)'; then
        consecutive_successes=$((consecutive_successes + 1))
        if [ "$consecutive_successes" -ge "$required_successes" ]; then
          return 0
        fi
      else
        consecutive_successes=0
      fi
    else
      consecutive_successes=0
    fi
    sleep 2
  done
  return 1
}

PUBLIC_HTTP_CODE=""
wait_for_public_http() {
  local max_attempts="$1"
  for _public_attempt in $(seq 1 "$max_attempts"); do
    PUBLIC_HTTP_CODE="$(curl -sS --connect-timeout 3 --max-time 5 "${PUBLIC_RESOLVE_ARGS[@]}" \
      -o /dev/null -w '%{http_code}' "$API_BASE/login" 2>/dev/null || true)"
    case "$PUBLIC_HTTP_CODE" in
      200|301|302|401 ) return 0 ;;
    esac
    sleep 2
  done
  return 1
}

cleanup() {
  local status=$?
  local rollback_needed=0
  local rollback_ok=1
  set +e

  if [ "$status" -ne 0 ] && [ "$DEPLOY_OK" -eq 0 ]; then
    if [ "$PROMOTED" -eq 1 ]; then
      rollback_needed=1
      echo "Deployment failed after source promotion; restoring the prior source." >&2
      if [ -d "$APP_DIR" ]; then
        if ! mv "$APP_DIR" "$FAILED_RELEASE"; then
          echo "ERROR: failed to retain the rejected release at $FAILED_RELEASE" >&2
          rollback_ok=0
        fi
      fi
      if [ -d "$BACKUP" ]; then
        if ! mv "$BACKUP" "$APP_DIR"; then
          echo "ERROR: failed to restore prior source $BACKUP" >&2
          rollback_ok=0
        fi
      else
        echo "ERROR: prior source backup is unavailable: $BACKUP" >&2
        rollback_ok=0
      fi
    fi
    if [ "$IMAGE_BUILT" -eq 1 ] && [ -n "$OLD_IMAGE_ID" ] && [ -n "$OLD_IMAGE_REF" ]; then
      rollback_needed=1
      echo "Restoring the prior application image." >&2
      if ! docker image tag "$ROLLBACK_IMAGE" "$OLD_IMAGE_REF"; then
        echo "ERROR: failed to restore prior image tag $OLD_IMAGE_REF" >&2
        rollback_ok=0
      fi
    fi
    if [ "$rollback_needed" -eq 1 ] && [ -n "$OLD_IMAGE_ID" ] && [ -n "$OLD_IMAGE_REF" ] && [ "$rollback_ok" -eq 1 ]; then
      if ! (
        cd "$STACK_DIR" &&
          docker compose -f docker-compose.yml -f docker-compose.docconvert.yml up -d --no-build orderapp
      ); then
        echo "ERROR: failed to restart the prior application release" >&2
        rollback_ok=0
      fi
    fi
    if [ "$rollback_needed" -eq 1 ] && [ "$rollback_ok" -eq 1 ]; then
      if wait_for_orderapp_http 15 1; then
        echo "Rollback verified: prior orderapp release is running and serving HTTP." >&2
      else
        echo "ERROR: rollback did not restore a healthy orderapp HTTP service" >&2
        rollback_ok=0
      fi
    fi
    if [ "$rollback_needed" -eq 1 ] && [ "$rollback_ok" -ne 1 ]; then
      echo "ERROR: automatic rollback is incomplete; inspect $ORDERAPP_CONTAINER immediately" >&2
    fi
  fi

  case "$BUILD_ROOT" in
    /tmp/kferp-orderapp-release-* ) rm -rf "$BUILD_ROOT" ;;
  esac
  if [ -n "$PREFLIGHT_IMAGE" ]; then
    if ! docker image rm "$PREFLIGHT_IMAGE" >/dev/null 2>&1; then
      echo "WARNING: could not remove temporary preflight image $PREFLIGHT_IMAGE" >&2
    fi
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [ ! -f "$VUE_DIR/package-lock.json" ] || [ ! -f "$MINIAPP_DIR/package-lock.json" ]; then
  echo "ERROR: npm lockfiles are required for reproducible remote builds" >&2
  exit 1
fi

# Keep all resource-heavy work on the server and deliberately serialize it.
export NODE_OPTIONS="${NODE_OPTIONS:---max-old-space-size=768}"
export npm_config_jobs="${npm_config_jobs:-1}"
export npm_config_audit=false
export npm_config_fund=false
export UV_THREADPOOL_SIZE="${UV_THREADPOOL_SIZE:-2}"
export GOMAXPROCS="${GOMAXPROCS:-1}"
export GOFLAGS="${GOFLAGS:--p=1}"
export COMPOSE_PARALLEL_LIMIT=1
export DOCKER_BUILDKIT=1

echo "[1/6] Installing and building Vue shell on the server..."
(
  cd "$VUE_DIR"
  nice -n 10 npm ci --no-audit --no-fund
  nice -n 10 node --test --test-concurrency=1 src/lib/*.test.js src/api/*.test.js
  VITE_KFERP_API_BASE="$API_BASE" nice -n 10 npm run build
)

echo "[2/6] Testing, type-checking and building mp-weixin on the server..."
(
  cd "$MINIAPP_DIR"
  nice -n 10 npm ci --no-audit --no-fund
  nice -n 10 npm test -- --maxWorkers=1 --minWorkers=1 --no-file-parallelism
  nice -n 10 npm run typecheck
  VITE_KFERP_API_BASE="$API_BASE" nice -n 10 npm run build:mp-weixin
  test -f dist/build/mp-weixin/app.json
  test -f dist/build/mp-weixin/project.config.json
  test -d dist/build/mp-weixin/pages
  if ! grep -R -Fq "$API_BASE" dist/build/mp-weixin; then
    echo "ERROR: mp-weixin artifact does not contain the expected API base: $API_BASE" >&2
    exit 1
  fi
  for order_entry_marker in 订单日期 商品 规格 数量 销售单价 选择客户后自动带入; do
    if ! grep -R -Fq "$order_entry_marker" dist/build/mp-weixin; then
      echo "ERROR: mp-weixin artifact is missing order-entry marker: $order_entry_marker" >&2
      exit 1
    fi
  done
  cat > dist/build/mp-weixin/RELEASE_INFO <<EOF
commit=$EXPECTED_COMMIT
environment=$TARGET_ENV
api_base=$API_BASE
built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF
)

echo "[3/6] Running Go tests with one build worker on the server..."
(
  cd "$RELEASE_DIR"
  nice -n 10 go test -p 1 ./...
)

# Copy only the built miniapp tree into the app release context. Dependencies
# stay in the temporary build tree and are removed before Docker sees it.
mkdir -p "$RELEASE_DIR/miniapp" "$RELEASE_DIR/docs/workspace"
tar --exclude='./node_modules' -C "$MINIAPP_DIR" -cf - . | tar -C "$RELEASE_DIR/miniapp" -xf -
for governance_doc in REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md DEPLOYMENT.md; do
  if [ -f "$SOURCE_ROOT/$governance_doc" ]; then
    cp "$SOURCE_ROOT/$governance_doc" "$RELEASE_DIR/docs/workspace/$governance_doc"
  fi
done
if [ -d "$SOURCE_ROOT/docs/acceptance" ]; then
  mkdir -p "$RELEASE_DIR/docs/acceptance"
  cp -R "$SOURCE_ROOT/docs/acceptance"/. "$RELEASE_DIR/docs/acceptance/"
fi

case "$VUE_DIR/node_modules" in "$BUILD_ROOT"/* ) rm -rf "$VUE_DIR/node_modules" ;; esac
case "$MINIAPP_DIR/node_modules" in "$BUILD_ROOT"/* ) rm -rf "$MINIAPP_DIR/node_modules" ;; esac
case "$RELEASE_DIR/miniapp/node_modules" in "$BUILD_ROOT"/* ) rm -rf "$RELEASE_DIR/miniapp/node_modules" ;; esac

# Dockerfile currently repeats Go tests. Use a release-only copy so that this
# second safety gate also stays at one Go worker without editing source files.
sed \
  -e 's#/usr/local/go/bin/go test \./\.\.\.#GOMAXPROCS=1 GOFLAGS=-p=1 /usr/local/go/bin/go test -p 1 ./...#' \
  -e 's#CGO_ENABLED=0 /usr/local/go/bin/go build -o#CGO_ENABLED=0 GOMAXPROCS=1 GOFLAGS=-p=1 /usr/local/go/bin/go build -p 1 -o#' \
  "$RELEASE_DIR/Dockerfile" > "$RELEASE_DIR/Dockerfile.remote-release"
if ! grep -q 'go test -p 1 ./\.\.\.' "$RELEASE_DIR/Dockerfile.remote-release"; then
  echo "ERROR: could not apply the single-worker Go test guard to the release Dockerfile" >&2
  exit 1
fi
if ! grep -q 'go build -p 1 -o' "$RELEASE_DIR/Dockerfile.remote-release"; then
  echo "ERROR: could not apply the single-worker Go build guard to the release Dockerfile" >&2
  exit 1
fi

if [ "$PREFLIGHT" -eq 1 ]; then
  PREFLIGHT_IMAGE="kferp-orderapp-preflight:$TARGET_ENV-$SHORT_COMMIT"
  echo "[4/4] Building an isolated preflight image without changing the deployment stack..."
  nice -n 10 docker build \
    --file "$RELEASE_DIR/Dockerfile.remote-release" \
    --tag "$PREFLIGHT_IMAGE" \
    "$RELEASE_DIR"
  DEPLOY_OK=1
  echo "Remote preflight completed. No source promotion or container restart was performed."
  echo "environment=$TARGET_ENV"
  echo "commit=$EXPECTED_COMMIT"
  exit 0
fi

if [ "$TARGET_ENV" = "development" ]; then
  "$SOURCE_ROOT/scripts/configure_public_ingress.sh" \
    "$SOURCE_ROOT/scripts/Caddyfile.public"
fi

# Preserve the legacy shipping export directory and optional template copy only
# for a real release. Feature-branch preflight never writes into STACK_DIR.
mkdir -p "$STACK_DIR/orderapp_data/shipping_exports"
if [ -f /data/ship_temp.xlsx ]; then
  cp /data/ship_temp.xlsx "$STACK_DIR/orderapp_data/ship_temp.xlsx"
fi

cat > "$BUILD_OVERRIDE" <<EOF
services:
  orderapp:
    build:
      context: $RELEASE_DIR
      dockerfile: Dockerfile.remote-release
EOF

cat > "$DOC_CONVERT_OVERRIDE" <<EOF
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
    container_name: $DOC_CONVERT_CONTAINER
    restart: unless-stopped
EOF

if docker inspect "$ORDERAPP_CONTAINER" >/dev/null 2>&1; then
  OLD_IMAGE_ID="$(docker inspect --format '{{.Image}}' "$ORDERAPP_CONTAINER")"
  OLD_IMAGE_REF="$(docker inspect --format '{{.Config.Image}}' "$ORDERAPP_CONTAINER")"
  docker image tag "$OLD_IMAGE_ID" "$ROLLBACK_IMAGE"
fi

echo "[4/6] Building the application image from the temporary server source..."
(
  cd "$STACK_DIR"
  nice -n 10 docker compose \
    -f docker-compose.yml \
    -f docker-compose.docconvert.yml \
    -f "$BUILD_OVERRIDE" \
    build orderapp
)
IMAGE_BUILT=1

echo "[5/6] Promoting release source and restarting only orderapp..."
if [ -e "$BACKUP" ]; then
  echo "ERROR: backup destination already exists: $BACKUP" >&2
  exit 1
fi
if [ -d "$APP_DIR" ]; then
  mv "$APP_DIR" "$BACKUP"
  PROMOTED=1
fi
mv "$RELEASE_DIR" "$APP_DIR"
PROMOTED=1

DOC_CONVERT_STATE="missing"
if docker inspect "$DOC_CONVERT_CONTAINER" >/dev/null 2>&1; then
  DOC_CONVERT_STATE="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$DOC_CONVERT_CONTAINER")"
fi
case "$DOC_CONVERT_STATE" in
  running|healthy|starting ) ;;
  * )
    (
      cd "$STACK_DIR"
      docker compose -f docker-compose.yml -f docker-compose.docconvert.yml up -d --pull missing --force-recreate docconvert
    )
    ;;
esac

(
  cd "$STACK_DIR"
  docker compose -f docker-compose.yml -f docker-compose.docconvert.yml up -d --no-build orderapp
)

# A running container can still be in a crash loop or need several seconds
# before the HTTP listener is ready. Poll for up to two minutes and require
# three consecutive live responses instead of treating Running=true as ready.
if ! wait_for_orderapp_http 60 3; then
  docker logs --tail 200 "$ORDERAPP_CONTAINER" >&2 || true
  echo "ERROR: orderapp container HTTP readiness failed; rollback will start" >&2
  exit 1
fi

if ! wait_for_public_http 15; then
  echo "ERROR: $API_BASE/login returned HTTP ${PUBLIC_HTTP_CODE:-none}; rollback will start" >&2
  exit 1
fi

# Require the same container to keep serving HTTP after both probes.
sleep 4
if ! wait_for_orderapp_http 1 1; then
  echo "ERROR: orderapp stopped serving HTTP after readiness; rollback will start" >&2
  exit 1
fi

DEPLOY_OK=1
echo "[6/6] Release completed."
echo "environment=$TARGET_ENV"
echo "commit=$EXPECTED_COMMIT"
echo "app_source=$APP_DIR"
echo "previous_source=$BACKUP"
echo "rollback_image=$ROLLBACK_IMAGE"
echo "miniapp_artifact=$APP_DIR/miniapp/dist/build/mp-weixin"
