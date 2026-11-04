import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../../..')
const checker = join(root, 'scripts/check_bom_start_rollback.sh')

function run(overrides = {}) {
  const dir = mkdtempSync(join(tmpdir(), 'pr748-rollback-'))
  try {
    writeFileSync(join(dir, 'docker'), `#!/bin/sh
case "$1" in
  image) printf '%s\\n' "$MOCK_LABEL" ;;
  inspect) printf 'DB_SCHEMA=%s\\n' "$MOCK_SCHEMA" ;;
  compose)
    [ "$MOCK_DB_DOWN" = 1 ] && exit 1
    case "$*" in *information_schema.tables*) printf '%s\\n' "$MOCK_TABLE" ;; *information_schema.columns*) printf '%s\\n' "$MOCK_COLUMN" ;; *) printf '%s\\n' "$MOCK_NEW" ;; esac ;;
  *) exit 1 ;;
esac
`, { mode: 0o755 })
    return spawnSync('bash', [checker, '/opt/stacks/erp', 'app', 'old-image'], {
      encoding: 'utf8',
      env: { ...process.env, PATH: `${dir}:${process.env.PATH}`, MOCK_LABEL: '', MOCK_SCHEMA: 'erp_test', MOCK_TABLE: 't', MOCK_COLUMN: 't', MOCK_NEW: 'f', MOCK_DB_DOWN: '0', ...overrides },
    })
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
}

test('rollback accepts an aware image and pauses new-policy writes', () => {
  const result = run({ MOCK_LABEL: '1', MOCK_DB_DOWN: '1' })
  assert.equal(result.status, 0)
  assert.equal(result.stdout.trim(), 'aware')
})

test('legacy rollback is allowed only when no new-policy order exists', () => {
  for (const env of [{ MOCK_COLUMN: 'f' }, { MOCK_NEW: 'f' }]) {
    const result = run(env)
    assert.equal(result.status, 0)
    assert.equal(result.stdout.trim(), 'legacy')
  }
  assert.notEqual(run({ MOCK_NEW: 't' }).status, 0)
})

test('rollback fails closed on unknown schema or unreachable database', () => {
  assert.notEqual(run({ MOCK_SCHEMA: 'bad-schema' }).status, 0)
  assert.notEqual(run({ MOCK_TABLE: 'f' }).status, 0)
  assert.notEqual(run({ MOCK_DB_DOWN: '1' }).status, 0)
})

test('release checks rollback before restoring the image and persists the write pause', () => {
  const release = readFileSync(join(root, 'scripts/remote_orderapp_release.sh'), 'utf8')
  const dockerfile = readFileSync(join(root, 'orderapp-remote/Dockerfile'), 'utf8')
  assert.ok(release.indexOf('docker stop "$ORDERAPP_CONTAINER"') < release.indexOf('check_bom_start_rollback.sh'))
  assert.ok(release.indexOf('check_bom_start_rollback.sh') < release.indexOf('docker image tag "$ROLLBACK_IMAGE"'))
  assert.match(release, /rollback_check_status" -ne 0/)
  assert.match(release, /KFERP_BOM_START_READ_ONLY: 1/)
  assert.match(dockerfile, /LABEL io\.kferp\.bom-start-aware="1"/)
})

test('failed release stops first and never restores an unsafe image', () => {
  for (const mode of ['blocked', 'aware']) {
    const dir = mkdtempSync(join(tmpdir(), 'pr748-release-'))
    try {
      const release = readFileSync(join(root, 'scripts/remote_orderapp_release.sh'), 'utf8')
      const cleanup = release.slice(release.indexOf('cleanup() {'), release.indexOf('trap cleanup EXIT'))
        .replace('exit "$status"', 'return "$status"')
      mkdirSync(join(dir, 'scripts'))
      mkdirSync(join(dir, 'app'))
      mkdirSync(join(dir, 'backup'))
      writeFileSync(join(dir, 'docker'), '#!/bin/sh\nprintf "%s\\n" "$*" >> "$TEST_DIR/calls"\nexit 0\n', { mode: 0o755 })
      writeFileSync(join(dir, 'sed'), '#!/bin/sh\n[ "$1" = -i ] || exit 1\n/usr/bin/sed "$2" "$3" > "$3.tmp" && /bin/mv "$3.tmp" "$3"\n', { mode: 0o755 })
      writeFileSync(join(dir, 'scripts/check_bom_start_rollback.sh'), '#!/bin/sh\n[ "$TEST_MODE" = blocked ] && exit 1\necho aware\n')
      writeFileSync(join(dir, 'override.yml'), '      KFERP_BOM_START_READ_ONLY: 0\n')
      const program = `${cleanup}\nwait_for_orderapp_http() { return 0; }\nfalse\ncleanup\n`
      const result = spawnSync('bash', ['-c', program], {
        encoding: 'utf8',
        env: { ...process.env, PATH: `${dir}:${process.env.PATH}`, TEST_DIR: dir, TEST_MODE: mode,
          DEPLOY_OK: '0', PROMOTED: '1', IMAGE_BUILT: '1', OLD_IMAGE_ID: 'old-id', OLD_IMAGE_REF: 'old-tag',
          ROLLBACK_IMAGE: 'saved-image', SOURCE_ROOT: dir, STACK_DIR: dir, ORDERAPP_CONTAINER: 'app',
          DOC_CONVERT_OVERRIDE: join(dir, 'override.yml'), APP_DIR: join(dir, 'app'), BACKUP: join(dir, 'backup'),
          FAILED_RELEASE: join(dir, 'failed'), BUILD_ROOT: dir, PREFLIGHT_IMAGE: '' },
      })
      assert.equal(result.status, 1)
      const calls = readFileSync(join(dir, 'calls'), 'utf8')
      assert.match(calls, /^stop app/m)
      if (mode === 'blocked') {
        assert.doesNotMatch(calls, /image tag/)
      } else {
        assert.match(calls, /image tag saved-image old-tag/)
        assert.match(readFileSync(join(dir, 'override.yml'), 'utf8'), /KFERP_BOM_START_READ_ONLY: 1/)
      }
    } finally {
      rmSync(dir, { recursive: true, force: true })
    }
  }
})
