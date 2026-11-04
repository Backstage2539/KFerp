#!/usr/bin/env bash
set -euo pipefail

# A pre-feature image cannot safely execute an order whose inputs were already
# posted at batch start. Unknown database state is treated as unsafe.
stack_dir="$1"
app_container="$2"
old_image="$3"

label="$(docker image inspect --format '{{index .Config.Labels "io.kferp.bom-start-aware"}}' "$old_image" 2>/dev/null || true)"
if [ "$label" = "1" ]; then
  echo aware
  exit 0
fi
schema="$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$app_container" | sed -n 's/^DB_SCHEMA=//p' | tail -1)"
if ! [[ "$schema" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
  echo 'Cannot verify the application database schema; refusing an unrecognized rollback image.' >&2
  exit 1
fi
query() {
  docker compose -f "$stack_dir/docker-compose.yml" exec -T postgres sh -c \
    'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "$1"' _ "$1"
}
has_table="$(query "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='$schema' AND table_name='work_orders')")"
if [ "$has_table" != t ]; then
  echo 'Cannot verify the work-order table; refusing rollback.' >&2
  exit 1
fi
has_column="$(query "SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='$schema' AND table_name='work_orders' AND column_name='material_consumption_policy')")"
case "$has_column" in
  f) echo legacy; exit 0 ;;
  t) ;;
  *) echo 'Cannot verify the consumption policy column; refusing rollback.' >&2; exit 1 ;;
esac
has_new="$(query "SELECT EXISTS(SELECT 1 FROM \"$schema\".work_orders WHERE material_consumption_policy='bom_start_v1')")"
if [ "$has_new" = f ]; then
  echo legacy
  exit 0
fi
echo 'New-policy work orders exist or their state cannot be verified; refusing an unrecognized rollback image.' >&2
exit 1
