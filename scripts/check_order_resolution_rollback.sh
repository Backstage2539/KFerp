#!/usr/bin/env bash
set -euo pipefail

# Older applications treat every funds step as merchandise cash. Never run
# them against fee-specific funds or an in-flight fee-only correction.
stack_dir="$1"
app_container="$2"
old_image="$3"
label="$(docker image inspect --format '{{index .Config.Labels "io.kferp.order-resolution-fee-aware"}}' "$old_image" 2>/dev/null || true)"
mixed_label="$(docker image inspect --format '{{index .Config.Labels "io.kferp.order-resolution-mixed-supply-aware"}}' "$old_image" 2>/dev/null || true)"
company_label="$(docker image inspect --format '{{index .Config.Labels "io.kferp.order-resolution-company-stock-aware"}}' "$old_image" 2>/dev/null || true)"
return_label="$(docker image inspect --format '{{index .Config.Labels "io.kferp.order-resolution-return-aware"}}' "$old_image" 2>/dev/null || true)"
if [ "$return_label" = 1 ] && [ "$label" = 1 ] && [ "$mixed_label" = 1 ] && [ "$company_label" = 1 ]; then exit 0; fi
schema="$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$app_container" | sed -n 's/^DB_SCHEMA=//p' | tail -1)"
if ! [[ "$schema" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
 echo 'Cannot verify the order resolution schema; refusing rollback.' >&2;exit 1
fi
query(){
 docker compose -f "$stack_dir/docker-compose.yml" exec -T postgres sh -c \
  'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "$1"' _ "$1"
}
has_table="$(query "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='$schema' AND table_name='order_resolution_requests')")"
case "$has_table" in f) exit 0;; t) ;; *) echo 'Cannot verify order resolution records; refusing rollback.' >&2;exit 1;; esac
if [ "$return_label" != 1 ]; then
 has_return="$(query "SELECT EXISTS(SELECT 1 FROM \"$schema\".order_resolution_requests r WHERE COALESCE(r.command_json->>'return_settlement','')<>'')")"
 if [ "$has_return" != f ]; then
  echo 'Physical return processing records exist or cannot be verified; refusing an incompatible rollback image.' >&2;exit 1
 fi
fi
if [ "$company_label" != 1 ]; then
 has_company="$(query "SELECT EXISTS(SELECT 1 FROM \"$schema\".order_resolution_requests r CROSS JOIN LATERAL jsonb_array_elements(COALESCE(r.command_json->'dispositions','[]'::jsonb)) d WHERE d->>'kind'='production' AND d->>'choice'='allocate' AND jsonb_array_length(COALESCE(d->'stock_selections','[]'::jsonb))>0)")"
 if [ "$has_company" != f ]; then
  echo 'Company stock assignment records exist or cannot be verified; refusing an incompatible rollback image.' >&2;exit 1
 fi
fi
if [ "$mixed_label" != 1 ]; then
 has_mixed="$(query "SELECT EXISTS(SELECT 1 FROM \"$schema\".order_resolution_requests r CROSS JOIN LATERAL jsonb_array_elements(COALESCE(r.command_json->'dispositions','[]'::jsonb)) d CROSS JOIN LATERAL jsonb_array_elements(COALESCE(d->'supplies','[]'::jsonb)) s WHERE s->>'physical_choice'='keep_reserved')")"
 if [ "$has_mixed" != f ]; then
  echo 'Mixed physical and pending supply records exist or cannot be verified; refusing an incompatible rollback image.' >&2;exit 1
 fi
fi
if [ "$label" = 1 ]; then exit 0; fi
has_new="$(query "SELECT EXISTS(SELECT 1 FROM \"$schema\".order_resolution_requests r WHERE r.command_json->>'scope'='fees_only' OR EXISTS(SELECT 1 FROM jsonb_array_elements(COALESCE(r.plan_json->'tasks','[]'::jsonb)) t WHERE t->>'kind'='funds' AND COALESCE((t->>'source_id')::bigint,0)>0))")"
if [ "$has_new" = f ]; then exit 0; fi
echo 'Fee-specific order resolution records exist or cannot be verified; refusing an incompatible rollback image. Keep a fee-aware release for recovery.' >&2
exit 1
