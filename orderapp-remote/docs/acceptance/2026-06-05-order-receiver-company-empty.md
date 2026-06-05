# PR-421 Order Receiver Company Empty

## Scope

- ERP 录单的收货单位允许留空。
- 新建订单保存时，收货姓名、电话、地址、收货单位空值统一写入空字符串，不写入 NULL。
- 修复 GoalE2E 四类商品订单在空收货单位下保存失败的问题。

## Evidence

- RED live API: `POST /app/api/order` with empty `receiver_company` failed with `SQLSTATE 23502`.
- RED local: `go test ./internal/infrastructure/postgres/sales -run TestSaveOrderReceiverFieldsUseNonNullText -count=1` failed before the fix.
- GREEN local: `go test ./internal/infrastructure/postgres/sales ./internal/interfaces/http/sales -count=1` passed.

## Manual Impact

- No user workflow change. The existing order entry flow stays the same; the fix removes an unintended save failure.
