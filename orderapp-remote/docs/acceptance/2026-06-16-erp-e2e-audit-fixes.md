# PR-498 ERP E2E Audit Fixes Acceptance

## Scope

- Browser audit from order entry through production planning, work orders, quality, stock trace, production logs, production costs, shipping and customer-facing order state.
- Fixes are intentionally narrow and preserve the existing ERPNext-style manufacturing boundaries: production plan, work order, job card, stock document, batch trace and order fulfillment stay separate.

## Expected Acceptance

- New order entry uses the local browser/server date, parses pasted recipient text with mobile phone priority, and only requires payment amount/voucher for `已收款`.
- Order list `只看可发货` includes production-complete, no-production and stock-ready orders but excludes already-shipped orders.
- Orders with producible lines and no persisted process status display `待计划` in the order list.
- Production plan opens on `待计划` demand by default; switching to all states changes the panel title to `生产需求`.
- Non-weight materials such as bags show purchase suggestions in material units instead of misleading 0g values.
- Auto-splitting operation capacity reports why no split was generated.
- Work orders derive expected output from planned grams/spec grams when unit fields are missing.
- Quality inspection shows localized work-order statuses and localized missing-field validation.
- Production costs hide empty trace placeholder rows.
- Production logs show finished batch and material batch evidence.
- Finished batch trace can recover missing material batch code from the matching production-run stock ledger.
- Hidden `mallSettings` route opens the mall management page; unknown `view` values show an unknown-page message instead of silently opening order entry.
- A blank local PostgreSQL schema can bootstrap the ERP app so browser acceptance can run from a clean database.

## Evidence

- Frontend unit: `node --test src/lib/order-entry.test.js src/lib/customer-recipient.test.js src/lib/order-shipping.test.js src/lib/local-date.test.js src/lib/view-routing.test.js`
- Frontend unit: `node --test src/lib/quality-inspections.test.js src/lib/work-orders.test.js src/lib/production-costs.test.js src/lib/produce-plan.test.js src/lib/production-logs.test.js`
- Sales API/repository: `go test ./internal/interfaces/http/sales -run 'TestOrdersShippingExcelAPIAcceptsNoProductionShipReadyOrders|TestOrderAPIShipReadyExcludesAlreadyShippedOrders' -count=1`; `go test ./internal/infrastructure/postgres/sales -run TestOrderProcessStatusExprDerivesUnplannedProductionDemand -count=1`
- Production repository: `go test ./internal/infrastructure/postgres/production -run 'TestMergeMaterialAvailabilityFallsBackToNonWeightPurchaseQuantity|TestProductionTraceAnalyticsOmitsEmptyTraceRows|TestListProductionLogsIncludesFinishedBatchCode' -count=1`
- Stock repository: `go test ./internal/infrastructure/postgres/stock -run TestGetStockTraceBackfillsBlankMaterialBatchFromLedger -count=1`
- Blank schema bootstrap: `ORDERAPP_TEST_DATABASE_URL=<temp-postgres> go test ./internal/appmain -run TestEnsureAppSchemaBootstrapsEmptyDatabase -count=1`

## Pending Release Checks

- Run broader targeted packages and frontend build.
- Run `scripts/verify_kferp.sh changed` and `git diff --check`.
- Reopen ERP in browser and verify the fixed pages against the original audit flow.
