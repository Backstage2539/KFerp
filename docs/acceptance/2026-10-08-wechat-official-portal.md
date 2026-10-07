# PR-684 公众号豆单与客户订单查询

- Branch: codex/wechat-official-portal, base develop 25bdce07.
- Product: review; Van business acceptance pending.
- DEV-737/738/739/740: implementation complete. DEV-741: development delivery in progress.
- Canonical manual: orderapp-remote/docs/OP_MANUAL_WECHAT_OFFICIAL.md; linked from customer portal/costing manuals and Vue help entry `wechatOfficialManual`.

## Implemented behavior

- One stable entry per logical price-table scope. Existing published tables backfilled; legacy snapshots without table identity kept separate; future first publications trigger entry creation. Default authenticated, manual pinning, independent scopes, optimistic revision saves, withdrawn/disabled fail closed. No historical snapshots/orders rewritten.
- Miniapp direct display, new read and permission check on each entry; no persistent private sheet cache. Existing customer certification, safe login return, official all-orders route with pagination beyond 50 (including processing customers).
- Trusted server UnionID resolution; manual one-time hashed code, five-minute expiry, concurrent single consumption, attempt limit, explicit multi-customer selection. Revocation tombstones prevent automatic rebind. Every order query checks live mini account, customer, binding, projected ERP account and capability.
- Encrypted callback validates signature, timestamp and AppID; replay binding requests are deduplicated, read replies recheck permission. Summary 1/3 orders uses existing sort/void scope, bounded UTF-8 text and item truncation.
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

Development release pending at the implementation commit. Exact commit, source backup, runtime health, authenticated smoke and feature markers will be recorded after release.

Not performed: production deployment of this feature, real service-account permission audit, credential entry, third-party message handover, WeChat miniapp upload/review/publish, actual official-account menu publish, real customer mobile end-to-end acceptance. The account remains disabled until setup. Backend readiness, miniapp release, menu publish, and phone test are separate required production steps.

Residual operational constraints: imported legacy action types unsupported by the ERP menu validator require manual editing or restoring the retained source menu in WeChat; API errors indicate missing account permissions/IP whitelist. Network timeout after publish requires re-importing current menu before retrying.
