# PR-327 Customer Responsible Employee

Date: 2026-05-22

## Scope
- 客户资料新增必填负责人，负责人只能是员工维护中的启用内部员工。
- 录单不再手工选择订单负责人；订单保存时忽略请求中的负责人字段，统一按客户资料负责人写入订单负责人快照。
- 录单新增客户、客户档案、销售单客户信息抽屉都要求选择客户负责人。
- 客户负责人变更写入操作日志，字段显示为“客户负责人”。

## Evidence
- `frontend-vue-shell/src/views/OrderEntryView.vue`：订单表单展示只读“客户负责人”，新增客户抽屉必填 `responsible_employee_id`。
- `frontend-vue-shell/src/lib/order-entry.js`：订单保存 payload 不再提交 `responsible_type` / `responsible_id`；负责人候选 helper 只返回员工。
- `internal/infrastructure/postgres/customer/repository.go`：客户新增、编辑、内联更新校验 `responsible_employee_id` 必填且为启用内部员工，变更写审计字段 `responsible_employee_id`。
- `internal/infrastructure/postgres/sales/repository.go`：订单保存从 `customer_id` 派生客户负责人，客户未配置负责人或负责人不是启用内部员工时拒绝保存。
- `internal/interfaces/http/support/audit_page.go`：操作日志字段 `responsible_employee_id` 显示为“客户负责人”。

## Verification
- `go test ./... -count=1` passed.
- `node --test src/lib/*.test.js src/api/*.test.js` passed: 286 tests.
- `npm run build` passed.
- `git diff --check` passed.

## Acceptance Notes
- PR-327 in `REQUIREMENTS.md` and `ACCEPTANCE_TESTS.md` covers the customer-profile source of truth, employee-only candidate set, required validation, order save behavior, and audit visibility.
- `OP_MANUAL_ORDER_SALES.md` documents the changed operator workflow and common failure handling.
