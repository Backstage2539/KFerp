# PR-292 用户操作审计留痕验收记录

## 范围
- 所有用户触发的业务写操作必须在操作日志可查。
- 本次先修复库存调整单缺口：库存数量调整和批次成本调整都写入 `stock_adjustment` 审计日志。
- 操作日志页面支持按“库存调整单”筛选，并用中文展示对象和字段。

## 验收口径
- 批次成本调整提交后，操作日志能看到操作者、库存调整单、批次号、旧成本、新成本、调整原因和价值变化。
- 库存数量调整提交后，操作日志能看到操作者、库存调整单、调整前数量、调整后数量、调整原因和调整批次号。
- 后续新增任何用户业务写操作时，不能只改业务数据而不写操作日志。

## 自动化证据
- `go test ./internal/infrastructure/postgres/stock -run 'TestStockAdjustmentsWriteAuditLogsSourceGuard|TestMaterialAdjustmentBackfillCreatesTransferableRawBatch|TestCreateAdjustmentUpdatesMaterialBatchUnitCost' -count=1`
- `go test ./internal/interfaces/http/support -run 'TestDecorateAuditLogRowScannedEntitiesUseReadableLabels|TestDev138AuditLogFilterIncludesReadableEntityTypes' -count=1`
- `go test ./internal/interfaces/http/stock -run 'TestStockAdjustmentsAPIRecordsMaterialCostAdjustment|TestStockAdjustmentsAPIRequiresReasonAndRecordsMaterialTarget' -count=1`

## 手册
- `OP_MANUAL_SETTINGS_AUDIT.md`
- `OP_MANUAL_INVENTORY_MATERIALS.md`
