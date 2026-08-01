#!/usr/bin/env bash
set -euo pipefail

SOURCE_FILE="${1:-}"
CADDY_FILE="/opt/stacks/erp-production/Caddyfile"
CADDY_CONTAINER="erp_prod_caddy"

case "$SOURCE_FILE" in
  /tmp/kferp-orderapp-release-*/repo/scripts/Caddyfile.public ) ;;
  * ) echo "ERROR: refusing unexpected public Caddy source: $SOURCE_FILE" >&2; exit 1 ;;
esac
if [ ! -f "$SOURCE_FILE" ]; then
  echo "ERROR: public Caddy source does not exist: $SOURCE_FILE" >&2
  exit 1
fi
if [ ! -f "$CADDY_FILE" ]; then
  echo "ERROR: public Caddy target does not exist: $CADDY_FILE" >&2
  exit 1
fi
if [ "$(docker inspect --format '{{.State.Running}}' "$CADDY_CONTAINER" 2>/dev/null || true)" != "true" ]; then
  echo "ERROR: public Caddy container is not running: $CADDY_CONTAINER" >&2
  exit 1
fi

CADDY_IMAGE="$(docker inspect --format '{{.Config.Image}}' "$CADDY_CONTAINER")"
docker run --rm \
  --volume "$SOURCE_FILE:/etc/caddy/Caddyfile:ro" \
  "$CADDY_IMAGE" \
  caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile

if cmp -s "$SOURCE_FILE" "$CADDY_FILE"; then
  echo "Public ingress already matches the release contract."
  exit 0
fi

BACKUP="${CADDY_FILE}.backup.domain-$(date +%Y%m%d%H%M%S)"
cp "$CADDY_FILE" "$BACKUP"
cp "$SOURCE_FILE" "$CADDY_FILE"

if ! docker exec "$CADDY_CONTAINER" \
  caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile; then
  echo "ERROR: public Caddy reload failed; restoring $BACKUP" >&2
  cp "$BACKUP" "$CADDY_FILE"
  docker exec "$CADDY_CONTAINER" \
    caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile || true
  exit 1
fi

echo "Public ingress updated without restarting application or database containers."
echo "previous_caddy=$BACKUP"
