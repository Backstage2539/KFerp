# PR-291 原料库存估值与批次成本调整验收记录

## 范围
- 成本试算和豆单价格来源中的生豆成本，改为读取 BOM 物料当前可用批次的加权平均成本。
- 入库价格录错时，通过库存调整单的“批次成本”修正原料批次成本，不直接修改物料档案价格。
- 原料数量补录生成新批次时，支持写入补录成本；未填写时回退物料默认采购价。

## 验收口径
- 加权平均公式：`Σ(批次当前可用克重 × 批次单位成本) ÷ Σ(批次当前可用克重)`。
- 只计算当前可用批次；待处理、冻结、不通过批次不参与。
- 批次成本调整只改变批次单位成本，并记录调整前成本、调整后成本和价值变化；库存数量不变。
- 批次成本调整必须在操作日志中按“库存调整单”可查，显示批次号、旧成本、新成本和调整原因。
- 没有可用批次时，成本试算才回退物料默认采购价。

## 自动化证据
- `go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsUsesAvailableBatchWeightedAverageBeanCost -count=1`
- `go test ./internal/application/stock -run 'TestCreateAdjustmentAcceptsMaterialCostAdjustment|TestCreateAdjustmentRejectsInvalidMaterialCostAdjustment' -count=1`
- `go test ./internal/interfaces/http/stock -run 'TestStockAdjustmentsAPIRecordsMaterialCostAdjustment|TestVueStockAdjustmentsExposeMaterialCostAdjustment' -count=1`
- `go test ./internal/infrastructure/postgres/stock -run 'TestCreateAdjustmentUpdatesMaterialBatchUnitCost|TestMaterialAdjustmentBackfillCreatesTransferableRawBatch' -count=1`
- `go test ./internal/infrastructure/postgres/stock -run 'TestStockAdjustmentsWriteAuditLogsSourceGuard|TestCreateAdjustmentUpdatesMaterialBatchUnitCost|TestMaterialAdjustmentBackfillCreatesTransferableRawBatch' -count=1`
- `go test ./...`
- `npm run build` in `orderapp-remote/frontend-vue-shell`
- Browser smoke on built Vue shell with mocked auth/API: opened `/vue-shell/?view=stockAdjustments`, confirmed `库存调整单`、`批次成本`、`补录成本/千克`，切换“批次成本”后确认 `原料批次`、`目标成本/千克`、`提交成本调整`。

## 手册
- `OP_MANUAL_INVENTORY_MATERIALS.md`
- `OP_MANUAL_COSTING.md`
