# PR-706 商品物流重量与无库存代发

- Branch: `codex/product-logistics-weight-20261020`; base `origin/develop@f9e10932`.
- Product acceptance: pending Van. Development technical delivery complete at `d0b75f4f9447f6c587673a3aa00f16907c43bca2` through PR #226.
- DEV-807: manual logistics weight on specification template and product BOM versions; positive publication gate; copies/reapply/logs.
- DEV-808: current-layer net material reference, manual logistics resolver, all-order frozen line weights and quote basis.
- DEV-809: no-stock UI, all-client detail weights, manuals and development delivery.

## Confirmed rules

Only direct KG/公斤/G/克 net materials count; no production loss or recursion. Manual logistics weight is mandatory for product specifications and is the sole freight basis. Drafts can omit it; new publications cannot. Existing historical orders remain unchanged. Quantity edits retain frozen unit weights; changing product/specification or copying creates fresh weights.

## Reproduction and RED

- Original on-demand view incorrectly included the zero-filled supply board.
- `/tmp/pr706-ui-red.log`: missing logistics editor/details and incorrect supply gating.
- `/tmp/pr706-bom-red.log`: missing API DTO field and no zero/negative validation.
- `/tmp/pr706-weight-red.log`: 0.227kg plus one bag failed to produce 227g.
- `/tmp/pr706-freight-red.log`: fixture manual 250g was ignored and existing spec relation returned 454g.
- `/tmp/pr706-orders-red.log`: order detail API omitted nullable frozen fields.

## Verification

- BOM and HTTP lifecycle/validation/copy/reapply/log tests: GREEN (`/tmp/pr706-bom-green.log`).
- Shared reference/logistics resolver and fractional-gram aggregation: GREEN (`/tmp/pr706-weight-pg.log`).
- ERP API initial save, quantity edit, stable-spec rename/version change, copy, changed spec and historical unknown: GREEN (`/tmp/pr706-orders-sales-positive.log`).
- Portal mall/product-order, stock, on-demand, explicit-identity import and mixed saved requests: GREEN (`/tmp/pr706-orders-portal-positive.log`, `/tmp/pr706-orders-request-green.log`, `/tmp/pr706-orders-import-green.log`).
- Explicit on-demand fixture: 12 items freeze reference 2724g and manual logistics 3000g despite 20% material loss; normal example 4 items freezes 908g/1000g.
- Whole Go suite without optional PostgreSQL: GREEN (`/tmp/pr706-go-final.log`).
- Final Vue suite: 1436 passed; miniapp: 371 passed. Both builds and miniapp typecheck passed (`/tmp/pr706-vue-final.log`, `/tmp/pr706-mini-final.log`).
- Narrow-screen/desktop actual-component visual checks: `/tmp/pr706-ui-mobile-375.png`, `/tmp/pr706-ui-desktop.png`; no live business-flow acceptance.
- Regression selection through 7 application/repository/HTTP packages: GREEN (`/tmp/pr706-integration-accepted.log`).
- Independent review fixed unknown-weight destination/config bypass, below-one-percent ratios, legacy bag/box dimensional ambiguity, stable-spec rename preservation and spec-editor state leakage. RED/GREEN evidence retained in `/tmp/pr706-*.log`.

### Existing optional PostgreSQL baseline failures

The full optional BOM PostgreSQL suite retains 11 existing failures; unmodified HEAD has 12, with no added failure. Comparisons: `/tmp/pr706-bom-final-full.log` and `/tmp/pr706-bom-baseline-full.log`. A broad selection also matched the unchanged legacy test `TestOrderAPISavesWholesale1000gByBeanListWeightTier`; original HEAD reproduces `400 product_bom_spec_not_configured` before the new weight hook (`/tmp/pr706-sales-baseline.log`). These are not reported as passing. All PR706 tests and relevant freight regressions pass. Dedicated local PostgreSQL database `kferp_pr706_test`; fixtures isolated by schema and cleaned. No live business data writes.

## Manuals

Canonical docs: OP_MANUAL_BOM_ORDER_PRICING.md, OP_MANUAL_PRODUCT_CREATOR.md, OP_MANUAL_ORDER_SALES.md, OP_MANUAL_CUSTOMER_STOCK_DROPSHIP.md, OP_MANUAL_CUSTOMER_FULFILLMENT.md. Existing OperationManualView loads these Markdown sources directly.

## Delivery boundary

Development environment and verified development miniapp export only. Production, WeChat upload/review/publication and final business acceptance are separate. No production logistics-weight backfill or guessed live configuration values.

## Pre-deployment backup

- Development database custom dump: `/opt/stacks/erp/backups/pr706-pre-logistics-weight-20261019T173934Z.dump` (18M); `pg_restore --list` passed.
- Additive nullable columns only. No historical weight backfill or live customer configuration writes.

## Final local gates and restart protection

- `/tmp/pr706-final-db.log`: shared weight, fulfillment/freight, portal and sales API selections passed after final fixes.
- `/tmp/pr706-go-final.log`: full Go passed after final source changes.
- `/tmp/pr706-orders-import-startup-green.log`: explicit spec imports, ambiguous imports unknown, quantity-only reimports after a new BOM publication preserve frozen weights; two EnsureSchema startups preserve row ID, all four weights and snapshot. Original failure captured in `/tmp/pr706-orders-startup-red.log`.
- Whole-order fractional grams are rounded once. Persisted ratio percentages 0.5 and 1 are not reinterpreted as fractional percentages.
- Independent review complete with no remaining confirmed blockers. PR remains review pending Van; technical release proceeds to development only.

## Development release recovery

- PR #226 merged verified source into develop at `d0b75f4f9447f6c587673a3aa00f16907c43bca2`.
- Initial server gates passed, but after promotion the shared root filesystem reached 100%; development PostgreSQL/app could not start (`SQLSTATE 53100`). Production containers remained running and production PostgreSQL healthy.
- The release script restored the previous source/image and verified HTTP readiness. Development PostgreSQL recovered through its normal WAL recovery; no database restore or business-data repair was performed.
- Cleared rebuildable host Go/npm caches, unused Docker build cache and dangling images only; retained running/rollback images, source backups, database dump and all data volumes. Available root space returned to 5.0GB.
- Refreshed and confirmed current origin/develop equals the same source SHA; reran the complete locked development release gates. Logs: `/tmp/pr706-deploy.log`, `/tmp/pr706-cache-space.log`, `/tmp/pr706-deploy-retry.log`. Final outcome follows below.

## Completed development release

- Locked retry completed successfully with full server Go tests, Vue 1436 tests/build, miniapp 371 tests/typecheck/development build and Docker Go/build gates (`/tmp/pr706-deploy-retry.log`). Runtime source is `d0b75f4f9447f6c587673a3aa00f16907c43bca2`; later delivery-receipt commits change documentation only.
- Source rollback anchor: `/opt/stacks/erp/orderapp.backup.deploy-20261020015920-d0b75f4f9447`; rollback image: `kferp-orderapp-rollback:development-20261020015920-d0b75f4f9447`.
- Read-only runtime checks: application running with zero restarts; PostgreSQL healthy; all seven new columns present; anonymous app HTTP 303, protected API HTTP 401; authenticated requirements and canonical manual HTTP 200. PR-706 is review; DEV-807/808/809 are done. Root filesystem has 3.7GB free. Evidence: `/tmp/pr706-smoke-final.log`.
- No live order submission, product weight configuration or historical data repair was performed. Van business acceptance remains pending. Existing published specifications without logistics weight must be copied to a new version, filled manually and published before nonfree automatic dropship freight can be calculated.

## Verified development miniapp package

- Export: `/Users/yiiiple-work/KFerp-logistics-weight-delivery-20261020/KFerp-miniapp-mp-weixin-dev`.
- ZIP: `/Users/yiiiple-work/KFerp-logistics-weight-delivery-20261020/KFerp-miniapp-dev-d0b75f4f.zip` (281,099 bytes, 237 files).
- SHA256: `2ba3d869ceee0ba35ae96264c612f200e081772876dcd005d18668d794c3d504`.
- RELEASE_INFO confirms the runtime commit, development environment and `https://dev.qacoohee.com/app`; no production API URL. All 24 pages and 96 required page files pass the manifest verifier. ZIP integrity and every entry's bytes match the exported files.
- Compiled weight fields, historical marker, both weight labels, scenario helper and supply/shortage/message gating checked. Evidence: `/tmp/pr706-artifact-final.log`.
- WeChat upload, review and publication and production deployment were not performed.
