#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT
old_app="$test_root/old-app"
new_app="$test_root/new-app"
artifact='miniapp/dist/build/mp-weixin'
mkdir -p "$old_app/$artifact/pages" "$new_app/miniapp"
printf '{"pages":[]}' > "$old_app/$artifact/app.json"
printf '{}' > "$old_app/$artifact/project.config.json"
printf 'commit=old-release\nenvironment=production\n' > "$old_app/$artifact/RELEASE_INFO"
printf 'original manifest\n' > "$old_app/$artifact/PAGE_FILE_MANIFEST"
printf 'original binary content\n' > "$old_app/$artifact/pages/old.js"
bash "$repo_root/scripts/retain_miniapp_artifact.sh" "$old_app" "$new_app"
diff -r "$old_app/$artifact" "$new_app/$artifact"
if bash "$repo_root/scripts/retain_miniapp_artifact.sh" "$test_root/missing-app" "$test_root/missing-destination" 2>/dev/null; then
  echo 'ERP-only release must reject a missing previous miniapp artifact' >&2
  exit 1
fi
test ! -e "$test_root/missing-destination/$artifact/RELEASE_INFO"
if bash "$repo_root/scripts/retain_miniapp_artifact.sh" "$old_app" "$new_app" >/dev/null 2>&1; then
  echo 'ERP-only release must not overwrite an existing target artifact' >&2
  exit 1
fi
mkdir -p "$test_root/incomplete/$artifact/pages"
printf '{}' > "$test_root/incomplete/$artifact/app.json"
if bash "$repo_root/scripts/retain_miniapp_artifact.sh" "$test_root/incomplete" "$test_root/incomplete-destination" >/dev/null 2>&1; then
  echo 'ERP-only release must reject an incomplete previous artifact' >&2
  exit 1
fi
test ! -e "$test_root/incomplete-destination/$artifact/RELEASE_INFO"
mkdir -p "$test_root/corrupt-bin"
cat > "$test_root/corrupt-bin/tar" <<'SH'
#!/usr/bin/env bash
/usr/bin/tar "$@"
if [ "${3:-}" = '-xf' ]; then printf 'corruption' >> "$2/pages/old.js"; fi
SH
chmod +x "$test_root/corrupt-bin/tar"
if PATH="$test_root/corrupt-bin:$PATH" bash "$repo_root/scripts/retain_miniapp_artifact.sh" "$old_app" "$test_root/corrupt-destination" >"$test_root/corrupt-output" 2>&1; then
  echo 'ERP-only release must reject a corrupted copied artifact' >&2
  exit 1
fi
grep -q 'SHA-256 verification' "$test_root/corrupt-output"
# Execute the actual conditional build block with stubbed build commands.
python3 - "$repo_root/scripts/remote_orderapp_release.sh" > "$test_root/build-phase.sh" <<'PY'
import pathlib,sys
source=pathlib.Path(sys.argv[1]).read_text()
start=source.index('if [ "$ERP_ONLY" = "1" ]; then\n  echo "[2/6]')
end=source.index('echo "[3/6]', start)
print(source[start:end])
PY
mkdir -p "$test_root/miniapp/dist/build/mp-weixin/pages"
printf 'https://erp.test 订单日期 商品 规格 数量 销售单价 选择客户后自动带入' > "$test_root/miniapp/dist/build/mp-weixin/app.json"
printf '{}' > "$test_root/miniapp/dist/build/mp-weixin/project.config.json"
export MINIAPP_DIR="$test_root/miniapp" SOURCE_ROOT="$test_root" API_BASE='https://erp.test' UNEXPECTED_API_BASE='https://other.test' EXPECTED_COMMIT='test-commit' TARGET_ENV='production' TEST_ROOT="$test_root"
nice() {
  printf '%s\n' "$*" >> "$TEST_ROOT/calls"
}
node() {
  [[ "$1" == *verify_mp_weixin_artifact.mjs ]] || return 1
  : > "$MINIAPP_DIR/dist/build/mp-weixin/PAGE_FILE_MANIFEST"
}
export -f nice node
ERP_ONLY=1 bash -e "$test_root/build-phase.sh"
test ! -e "$test_root/calls"
test ! -e "$MINIAPP_DIR/dist/build/mp-weixin/RELEASE_INFO"
ERP_ONLY=0 bash -e "$test_root/build-phase.sh"
grep -q 'npm run build:mp-weixin' "$test_root/calls"
grep -q '^commit=test-commit$' "$MINIAPP_DIR/dist/build/mp-weixin/RELEASE_INFO"
# Exercise the actual local entrypoint with isolated fake git/ssh commands.
mkdir -p "$test_root/fake-bin"
cat > "$test_root/fake-bin/git" <<'SH'
#!/usr/bin/env bash
case "$*" in
  'rev-parse --show-toplevel') printf '%s\n' "$TEST_REPO_ROOT" ;;
  'rev-parse --abbrev-ref HEAD') echo codex/test ;;
  'rev-parse --abbrev-ref --symbolic-full-name @{upstream}') echo origin/test ;;
  'rev-parse HEAD'|'rev-parse origin/test') echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa ;;
  'remote get-url origin') echo fake-origin ;;
  'status --porcelain'|'fetch --quiet origin') : ;;
  'archive '* ) /usr/bin/tar -cf - --files-from /dev/null ;;
  *) printf 'Unexpected git arguments: %s\n' "$*" >&2; exit 1 ;;
esac
SH
cat > "$test_root/fake-bin/ssh" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$TEST_SSH_LOG"
case "$*" in *'tar -C'*) cat >/dev/null ;; esac
SH
chmod +x "$test_root/fake-bin/git" "$test_root/fake-bin/ssh"
export TEST_REPO_ROOT="$repo_root" TEST_SSH_LOG="$test_root/ssh.log"
PATH="$test_root/fake-bin:$PATH" KFERP_ERP_ONLY=1 KFERP_DEPLOY_SERVER=fake bash "$repo_root/deploy_orderapp.sh" --preflight production > "$test_root/erp-entry-output"
grep -q -- '--erp-only' "$test_root/ssh.log"
: > "$test_root/ssh.log"
PATH="$test_root/fake-bin:$PATH" KFERP_ERP_ONLY=0 KFERP_DEPLOY_SERVER=fake bash "$repo_root/deploy_orderapp.sh" --preflight production > "$test_root/full-entry-output"
if grep -q -- '--erp-only' "$test_root/ssh.log"; then echo 'default preflight must not pass ERP-only flag' >&2; exit 1; fi
: > "$test_root/ssh.log"
if PATH="$test_root/fake-bin:$PATH" KFERP_ERP_ONLY=invalid KFERP_DEPLOY_SERVER=fake bash "$repo_root/deploy_orderapp.sh" --preflight production >/dev/null 2>&1; then echo 'invalid ERP-only value must fail' >&2; exit 1; fi
test ! -s "$test_root/ssh.log"

python3 - "$repo_root" <<'PY'
import pathlib,sys
root=pathlib.Path(sys.argv[1])
entry=(root/'deploy_orderapp.sh').read_text()
remote=(root/'scripts/remote_orderapp_release.sh').read_text()
assert 'KFERP_ERP_ONLY' in entry and 'remote_release_args+=(--erp-only)' in entry
assert '--erp-only' in remote and 'retain_miniapp_artifact.sh' in remote
assert remote.index('flock -n 9') < remote.index('retain_miniapp_artifact.sh') < remote.index('Promoting release source')
assert 'KFERP_SKIP_MINIAPP_EXPORT' in entry and 'ERP_ONLY' in entry[entry.index('sync_miniapp_artifact()'):]
assert 'flock -n 9' in remote and 'wait_for_orderapp_http' in remote and 'check_bom_start_rollback.sh' in remote
PY
printf 'ERP-only artifact retention and release gates: PASS\n'
