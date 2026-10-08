# PR-684 公众号豆单与客户订单查询

- Branch: codex/wechat-official-portal-ready, base develop 25bdce07.
- Product: review; Van business acceptance pending.
- DEV-737/738/739/740/741: implementation and development delivery complete.
- Canonical manual: orderapp-remote/docs/OP_MANUAL_WECHAT_OFFICIAL.md; linked from customer portal/costing manuals and Vue help entry `wechatOfficialManual`.

## Implemented behavior

- One stable entry per logical price-table scope. Existing published tables backfilled; legacy snapshots without table identity kept separate; future first publications trigger entry creation. Default authenticated, manual pinning, independent scopes, optimistic revision saves, withdrawn/disabled fail closed. No historical snapshots/orders rewritten.
- Miniapp direct display, new read and permission check on each entry; no persistent private sheet cache. Existing customer certification, safe login return, official all-orders route with pagination beyond 50 (including processing customers).
- Trusted server UnionID resolution; manual one-time hashed code, five-minute expiry, concurrent single consumption, attempt limit, explicit multi-customer selection. Revocation tombstones prevent automatic rebind. Every order query checks live mini account, customer, binding, projected ERP account and capability.
- Encrypted callback validates signature, timestamp and AppID; replay binding requests are deduplicated, read replies recheck permission. Summary 1/3 orders uses existing sort/void scope, bounded UTF-8 text and item truncation. A worst-case 2,074-byte regression now stays under the WeChat 2,048-byte limit, retains three order headers and the full-order link hint.
- ERP config status only (no secrets), stable entry settings, binding revoke, import/save menu drafts, full current/proposed comparison, signed short-lived preview, publish with previous menu snapshot, history load for recovery, audit events.
- Configuration flags default off. Remote deploy forwards server-side WeChat env only. No actual WeChat API calls or publishing performed during implementation.

## RED → GREEN evidence

Initial tests failed on missing entry/menu/summary interfaces, missing pagination fields and missing miniapp login-return helper. New regression tests also failed for dropped pagination during normalization, accepting an incomplete callback configuration, inability to disable a withdrawn entry, and reactivating a revoked binding through selection. Corrected and re-run successfully.

| Verification | Evidence |
| --- | --- |
| Go full backend | scripts/verify_kferp.sh backend passed; database-dependent existing tests skip without DSN |
| Real PostgreSQL plus race detector | ORDERAPP_TEST_DATABASE_URL to disposable local DB; go test ./internal/appmain -run '^TestWechatOfficialPostgres$' -race -count=1 -v; eight scenarios pass |
| Scope/version permissions | New published versions keep old path/pin; other table switch denied; private cannot public; stale revision denied; withdrawn unavailable; migration preserves disabled state; mini resolver no-store and live authorization |
| Identity and codes | Trusted UnionID vs OpenID, multi-customer no guess, revoke tombstone, simultaneous consumption exactly once, expired/limited/pending/disabled denied, capability removed immediately |
| Orders and encrypted callback | 55 orders + other-customer and void records; offset 40 returns remaining 15, recent 1/3 correct; real AES/XML request and reply; duplicate binding handled; same recent callback after revoke contains no order |
| Vue | Full frontend gate 1,325 assertions passed, Vite build passed |
| Miniapp | Typecheck passed; 267 tests passed; development WeChat build passed |
| UI review | Local Chrome fixture only: configuration, fixed path, price/gradient preview, menu editor, current/proposed target paths, publishing disabled before setup; no live business acceptance |
| Whitespace/build | git diff --check; standard changed gate and server release gates before delivery |

## Delivery and acceptance boundary

PR [#185](https://github.com/Backstage2539/KFerp/pull/185) merged the pushed feature head `50ab9e719eaecdc02e7e3f5533000222d30bc866` into develop at `b1521ba30d61b4398f400b9602e343c5fa4db3aa`. Deployed that exact develop commit on 2026-10-08 with `KFERP_MINIAPP_DEVELOPMENT_EXPORT_DIR=/Users/yiiiple-work/Documents/Codex/2026-10-06/main/outputs/KFerp-miniapp-mp-weixin-dev ./deploy_orderapp.sh development`.

- Source backup: `/opt/stacks/erp/orderapp.backup.deploy-20261008011249-b1521ba30d61`.
- Rollback image: `kferp-orderapp-rollback:development-20261008011249-b1521ba30d61`.
- Running application image: `sha256:062795722424c6c2af8f506765b41db4c8c817e201cdd5ee471481563b50ad88`; started `2026-10-07T17:20:14Z`, running with zero restarts, clean listener startup. PostgreSQL healthy; the shared public Caddy (`erp_prod_caddy`) running. The dormant development Caddy remains stopped by the existing shared-ingress configuration.
- Final server Vue, miniapp, Go and container build gates passed. External TLS login smoke returned 200. Existing browser routing is `/app/` 303 to orders, unauthenticated orders 302 to login (this environment already uses `DISABLE_BASIC_AUTH=true`); protected anonymous admin/binding APIs returned 401. Authenticated official status/entries/manual requests returned 200. Missing fixed entry returned 404. Disabled callback returned 503.
- Official configuration remains disabled, credentials unconfigured, UnionID and menu publication switches off. Entry backfill returned 30 rows, all authenticated by default. No live menu publication or customer binding occurred. PR-684 remains review; DEV-741 delivery metadata was marked done after successful smoke.
- Miniapp `RELEASE_INFO` matches deployed commit, `environment=development`, `api_base=https://dev.qacoohee.com/app`. Verified 19 declared pages and 76 page files including `pages/price-list/price-list`. Export directory: `/Users/yiiiple-work/Documents/Codex/2026-10-06/main/outputs/KFerp-miniapp-mp-weixin-dev`. ZIP: `KFerp-miniapp-wechat-dev-b1521ba3.zip`, 174 files, SHA256 `eecf52cbc63fe8ca370f25c9b0145b92b101362e91ea4eb1ba8fa15efdb5b420`.
- The subsequent receipt commit only updates tracking and evidence. The deployed application remains the release commit above; its bootstrap delivery seed predates the receipt, so an isolated restart of this older image can display DEV-741 as doing until the next normal code deployment. No business feature differs.

Not performed: production deployment of this feature, real service-account permission audit, credential entry, third-party message handover, WeChat miniapp upload/review/publish, actual official-account menu publish, real customer mobile end-to-end acceptance. The account remains disabled until setup. Backend readiness, miniapp release, menu publish, and phone test are separate required production steps.

Residual operational constraints: imported legacy action types unsupported by the ERP menu validator require manual editing or restoring the retained source menu in WeChat; API errors indicate missing account permissions/IP whitelist. Network timeout after publish requires re-importing current menu before retrying.
