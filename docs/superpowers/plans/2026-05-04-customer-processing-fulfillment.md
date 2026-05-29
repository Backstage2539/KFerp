# Customer Processing Fulfillment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first customer-owned fulfillment loop: remove the standalone logistics entry, configure customer processing warehouses/senders, turn processing shipment requests into normal orders, and make processing production requests visible in the production plan.

**Architecture:** Treat `orders.customer_id` as the upstream customer/source owner and store downstream recipient data as order snapshots. Customer portal profiles own default processing warehouse and default sender settings. Processing requests become production demand records; processing shipment requests create ordinary orders tagged by portal service and sourced from the customer warehouse.

**Tech Stack:** Go application services and PostgreSQL repositories under `orderapp-remote/internal`, Vue/Vite ERP admin under `orderapp-remote/frontend-vue-shell`, uni-app miniapp under `miniapp`, tests via `go test`, `node --test`, `npm test --prefix miniapp`, and `npm run build`.

---

## File Structure

- Modify `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`: add portal profile warehouse/sender fields, processing production demand fields, and processing shipment request table.
- Modify `orderapp-remote/internal/application/customerportal/service.go`: add customer portal profile config DTOs, processing shipment command, and service authorization/validation.
- Modify `orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository.go`: persist processing warehouse and sender profile from ERP customer portal settings.
- Modify `orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go`: create processing requests as production demand, create processing shipment orders, and list customer inventory from real customer warehouses.
- Modify `orderapp-remote/internal/infrastructure/postgres/core/schema.go`: add order recipient snapshot and portal source columns.
- Modify `orderapp-remote/internal/infrastructure/postgres/sales/*`: make customer order queries, shipping export, delivery notes, and stock deduction use order recipient snapshots and per-order source warehouse.
- Modify `orderapp-remote/internal/infrastructure/postgres/production/*`: include processing production demand in production plan and finish processing work into customer warehouse.
- Modify `orderapp-remote/internal/interfaces/http/customerportal/*`: expose processing shipment submit API and pass service filters.
- Modify `miniapp/src/utils/capabilities.ts` and `miniapp/src/utils/servicePage.ts`: remove standalone logistics entry and keep orders as the logistics lookup surface.
- Modify `miniapp/src/pages/service/service.vue`: add processing shipment form and show processing production/order status in the processing service.
- Modify `orderapp-remote/frontend-vue-shell/src/views/CustomerPortalSettingsView.vue`: add inline processing warehouse and sender controls.
- Modify `orderapp-remote/internal/interfaces/http/support/req_store.go`: add PR/DEV/UT/API/REV rows for this requirement.

## Task 1: Requirement Rows And Logistics Entry Removal

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Modify: `miniapp/src/utils/capabilities.ts`
- Modify: `miniapp/src/utils/servicePage.ts`
- Modify: `miniapp/src/pages/login/login.vue`
- Test: `miniapp/src/utils/capabilities.test.ts`
- Test: `miniapp/src/utils/servicePage.test.ts`
- Test: `orderapp-remote/internal/interfaces/http/support/dev_customer_portal_processing_fulfillment_test.go`

- [ ] **Step 1: Write failing miniapp tests**

Add assertions that `visibleHomeEntries` never returns key `shipping`, while the `orders` entry remains visible for `product_order`, `direct_ship`, or future order-capable customers.

Run: `npm test --prefix miniapp -- capabilities.test.ts servicePage.test.ts`

Expected: FAIL because `shipping` is still defined and `servicePage` still treats `shipping` as a normal service key.

- [ ] **Step 2: Write failing support requirement seed test**

Create `orderapp-remote/internal/interfaces/http/support/dev_customer_portal_processing_fulfillment_test.go` with source guards that require these codes in `req_store.go`:

```go
PR-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT
DEV-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-01
DEV-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-02
DEV-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-03
UT-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-01
API-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-01
REV-CUSTOMER-PORTAL-PROCESSING-FULFILLMENT-01
```

Run: `go test ./internal/interfaces/http/support -run TestCustomerPortalProcessingFulfillmentRequirements -count=1`

Expected: FAIL because the requirement rows do not exist.

- [ ] **Step 3: Implement logistics removal and requirement rows**

Remove the `shipping` home entry from `miniapp/src/utils/capabilities.ts`, remove `shipping` from `ServiceKey` and service labels in `miniapp/src/utils/servicePage.ts`, and update login copy to say customers can view bean lists, orders, inventory, and settlement services. Keep backend `shipping` service compatibility until all older miniapp builds are gone.

Add requirement seed rows in `req_store.go`:

- PR: customer-owned processing fulfillment loop.
- DEV-01: hide standalone logistics entry and keep logistics in orders.
- DEV-02: customer portal profile stores processing warehouse/default sender and creates a real warehouse.
- DEV-03: processing requests create production demand; processing shipment requests create customer-owned orders with recipient snapshots.
- UT: service/schema/miniapp/source-guard coverage.
- API: mini API tests for processing submit and shipment order creation.
- REV: Van verifies ERP settings, miniapp processing work order, miniapp processing shipment order, and order logistics in order detail.

Run: `npm test --prefix miniapp -- capabilities.test.ts servicePage.test.ts && go test ./internal/interfaces/http/support -run TestCustomerPortalProcessingFulfillmentRequirements -count=1`

Expected: PASS.

## Task 2: Customer Portal Profile Warehouse And Sender Config

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomerPortalSettingsView.vue`
- Test: `orderapp-remote/internal/infrastructure/postgres/customerportal/schema_test.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository_source_test.go`
- Test: `orderapp-remote/internal/application/customerportal/service_test.go`

- [ ] **Step 1: Write failing backend tests**

Extend schema/admin/service tests to require:

- `customer_portal_profiles.processing_warehouse_code`
- `customer_portal_profiles.default_sender_id`
- `PortalAdminDetail.customer.processing_warehouse_code`
- `PortalAdminDetail.customer.default_sender_id`
- `UpdatePortalVisibility` trims warehouse code and preserves default sender id.

Run: `go test ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal -count=1`

Expected: FAIL because the fields are missing.

- [ ] **Step 2: Implement backend profile fields**

Add columns:

```sql
ALTER TABLE customer_portal_profiles ADD COLUMN IF NOT EXISTS processing_warehouse_code TEXT NOT NULL DEFAULT '';
ALTER TABLE customer_portal_profiles ADD COLUMN IF NOT EXISTS default_sender_id BIGINT NOT NULL DEFAULT 0;
```

Extend DTOs and repository reads/writes. Normalize empty warehouse to `cust_<customer_id>_processing` when processing capability is enabled and no warehouse is configured. Insert or update `warehouses(code,name,kind,parent_code,sort_order,is_default,active,description)` with `kind='customer_processing'`, parent `finished_goods`, and name `<display/customer name>-代加工仓`.

Run: `go test ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal -count=1`

Expected: PASS.

- [ ] **Step 3: Add ERP UI controls**

In the customer portal settings list row, add compact inputs/selects for processing warehouse code and default sender id. Reuse the existing row form/save flow, and keep capability configuration in the list row.

Run: `node --test src/lib/*.test.js src/api/*.test.js` from `orderapp-remote/frontend-vue-shell`.

Expected: PASS.

## Task 3: Order Recipient Snapshot And Customer Source Fields

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/core/schema.go`
- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/shipping_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_shipping_export_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/delivery_note_repository.go`
- Test: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/sales/sales_order_repository_test.go`

- [ ] **Step 1: Write failing sales/order tests**

Add tests proving an order can carry:

```json
{
  "portal_service_code": "processing_ship",
  "source_warehouse": "cust_147_processing",
  "receiver_name": "张三",
  "receiver_phone": "13800000001",
  "receiver_address": "上海市测试路1号"
}
```

The order list, shipping query, delivery note, and shipping export must show receiver fields from the order snapshot, not from `customers`.

Run: `go test ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales -run 'Test.*Receiver|Test.*PortalService|Test.*DeliveryNote' -count=1`

Expected: FAIL because the fields do not exist or queries still read only `customers`.

- [ ] **Step 2: Implement order columns and query fallback**

Add order columns:

```sql
receiver_name TEXT NOT NULL DEFAULT '',
receiver_phone TEXT NOT NULL DEFAULT '',
receiver_address TEXT NOT NULL DEFAULT '',
receiver_company TEXT NOT NULL DEFAULT '',
portal_service_code TEXT NOT NULL DEFAULT '',
source_warehouse TEXT NOT NULL DEFAULT ''
```

Update queries to use `COALESCE(NULLIF(o.receiver_name,''), NULLIF(c.contact,''), c.name, '')`, same fallback pattern for phone/address/company. Use `o.source_warehouse` for delivery/shipping source when present; fallback to saved delivery note form; then fallback to `finished_goods`.

Run: `go test ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales -count=1`

Expected: PASS.

## Task 4: Processing Request Creates Production Demand

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/unprod_summary.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/running_repository.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository_test.go`
- Test: `orderapp-remote/internal/interfaces/http/production/produce_plan_api_test.go`

- [ ] **Step 1: Write failing processing-production tests**

Test that `CreateProcessingRequest` creates a demand row linked to the request:

- customer id is the upstream customer.
- product/spec/qty become need grams.
- target warehouse is the customer's processing warehouse.
- production plan includes this need even without a sales order.

Run: `go test ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/production -run 'TestProcessing.*Production|TestProducePlan.*Processing' -count=1`

Expected: FAIL because there is no production demand table or plan query integration.

- [ ] **Step 2: Implement production demand table and plan union**

Create `customer_processing_production_demands` with request id, customer id, product id, spec g, target qty, need g, target warehouse, status, linked batch/running/work order ids.

On processing request creation, insert one demand row with status `planned`. In `fetchUnproducedNeeds`, union demand rows with sales order needs and use demand request no in `order_nos`. In production start, mark matched demand rows `running`. On production finish, if the running item came from processing demand, finish into the demand target warehouse and mark demand `done`.

Run: `go test ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/production -count=1`

Expected: PASS.

## Task 5: Processing Shipment Request Creates Customer Order

**Files:**
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`
- Modify: `miniapp/src/api/customerPortal.ts`
- Modify: `miniapp/src/pages/service/service.vue`
- Test: `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`
- Test: `miniapp/src/utils/servicePage.test.ts`

- [ ] **Step 1: Write failing API and miniapp tests**

Add a mini API test for:

`POST /api/mini/processing-shipments`

Payload:

```json
{
  "receiver_name": "张三",
  "receiver_phone": "13800000001",
  "receiver_address": "上海市测试路1号",
  "items": [{"product_id": 5, "spec_g": 454, "qty": 2}],
  "shipping_amount": "12.00",
  "direct_ship_fee": "3.00",
  "note": "代加工客户发货"
}
```

Expected JSON includes `order_id`, `order_no`, `portal_service_code=processing_ship`, and the created order belongs to the current customer.

Run: `go test ./internal/interfaces/http/customerportal -run TestMiniProcessingShipmentCreatesCustomerOrder -count=1 && npm test --prefix miniapp`

Expected: FAIL because API and UI do not exist.

- [ ] **Step 2: Implement processing shipment command**

Create `processing_shipment_requests` for portal audit. Insert into `orders` with customer id, source `小程序`, type default/customer default, receiver snapshot, `portal_service_code='processing_ship'`, `source_warehouse=<customer processing warehouse>`, shipping amount, and notes. Insert order items. Insert customer fee rows for shipping/direct ship service fees when amounts are positive.

Run: `go test ./internal/interfaces/http/customerportal -count=1 && npm test --prefix miniapp`

Expected: PASS.

## Task 6: Stock Deduction Uses Order Source Warehouse

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_stock_deductions.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_stock_batches.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/delivery_note_repository.go`
- Test: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`

- [ ] **Step 1: Write failing stock deduction test**

Seed finished inventory and stock ledger entries in `cust_147_processing`, allocate an order item from that batch, mark the order shipped, and assert stock deduction records ledger entries from `cust_147_processing`.

Run: `go test ./internal/interfaces/http/sales -run TestOrderShipmentDeductsCustomerWarehouseStock -count=1`

Expected: FAIL because deduction requires `finished_goods`.

- [ ] **Step 2: Implement source warehouse deduction**

Resolve order source warehouse from `orders.source_warehouse`, then delivery note form, then `finished_goods`. Replace hard-coded `finished_goods` checks in order stock deduction with the resolved warehouse. Keep legacy fallback for orders without source warehouse.

Run: `go test ./internal/interfaces/http/sales -run TestOrderShipmentDeductsCustomerWarehouseStock -count=1`

Expected: PASS.

## Task 7: Full Verification And Acceptance Evidence

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Update: `memory/2026-05-04.md` if useful for deployment notes.

- [ ] **Step 1: Run backend unit/API suites**

Run:

```bash
go test ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/customerportal ./internal/interfaces/http/sales ./internal/interfaces/http/production ./internal/interfaces/http/support -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 2: Run frontend and miniapp suites**

Run:

```bash
npm test --prefix miniapp
npm run typecheck --prefix miniapp
VITE_KFERP_API_BASE=https://erp.qacoohee.com/app npm run build:mp-weixin --prefix miniapp
cd orderapp-remote/frontend-vue-shell && node --test src/lib/*.test.js src/api/*.test.js && npm run build
```

Expected: PASS.

- [ ] **Step 3: Acceptance review**

Confirm evidence against PR:

- Miniapp home no longer has standalone logistics.
- Orders contain logistics state and tracking.
- ERP customer portal row can configure processing warehouse and sender.
- Processing request creates production demand.
- Processing shipment creates an order owned by the current customer with downstream receiver snapshots.
- Shipping/delivery uses the configured customer sender and customer processing warehouse.

Update PR/DEV/UT/API evidence in `req_store.go` from `todo` to `review/done` as appropriate.

- [ ] **Step 4: Commit**

Run:

```bash
git diff --check
git status --short
git add docs/superpowers/plans/2026-05-04-customer-processing-fulfillment.md orderapp-remote miniapp
git commit -m "feat: add customer processing fulfillment flow"
```

Expected: commit succeeds on `codex/customer-processing-fulfillment-20260504`.

## Self-Review

- Spec coverage: logistics entry removal, customer-owned warehouse, customer source distinction, sender ownership, processing production plan integration, processing shipment into orders, and settlement fee generation are all mapped to tasks.
- Placeholder scan: no `TBD` or deferred behavior is used; unknown implementation details are expressed as concrete tests and expected data fields.
- Type consistency: `processing_warehouse_code`, `default_sender_id`, `portal_service_code`, `source_warehouse`, and `receiver_*` are consistently named across schema, DTO, API, and miniapp tasks.
