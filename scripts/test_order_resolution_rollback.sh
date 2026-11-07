#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
test_dir="$(mktemp -d /tmp/kferp-resolution-rollback.XXXXXX)"
trap 'rm -rf "$test_dir"' EXIT
cat > "$test_dir/docker" <<'MOCK'
#!/usr/bin/env bash
case "$*" in
 *'image inspect'*) echo "${ROLLBACK_FEE_AWARE:-0}" ;;
 *'inspect --format'*) echo DB_SCHEMA=p2rms15pepb5ciz ;;
 *information_schema.tables*) echo t ;;
 *command_json*) echo "${ROLLBACK_NEW_DATA:-t}" ;;
 *) exit 7 ;;
esac
MOCK
chmod +x "$test_dir/docker"
if PATH="$test_dir:$PATH" ROLLBACK_FEE_AWARE=0 ROLLBACK_NEW_DATA=t bash "$root/scripts/check_order_resolution_rollback.sh" /test app old > "$test_dir/result" 2>&1;then
 echo 'FAIL: legacy rollback accepted fee-resolution records';exit 1
fi
if ! test -f "$root/scripts/check_order_resolution_rollback.sh";then echo 'FAIL: rollback guard missing';exit 1;fi
PATH="$test_dir:$PATH" ROLLBACK_FEE_AWARE=0 ROLLBACK_NEW_DATA=f bash "$root/scripts/check_order_resolution_rollback.sh" /test app old
PATH="$test_dir:$PATH" ROLLBACK_FEE_AWARE=1 ROLLBACK_NEW_DATA=t bash "$root/scripts/check_order_resolution_rollback.sh" /test app old
printf 'PASS: fee-aware rollback and unchanged-data rollback; legacy with new records rejected\n'
