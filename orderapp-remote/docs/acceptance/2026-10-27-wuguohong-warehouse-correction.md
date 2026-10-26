# 吴国宏仓库库存按批次更正验收

日期：2026-10-27  
需求：PR-737-WAREHOUSE-BATCH-STOCK-ADJUSTMENT  
开发任务：DEV-880-FINISHED-BATCH-ADJUSTMENT、DEV-881-WAREHOUSE-CORRECTION-DELIVERY

## 验收目标

吴国宏仓库最终只保留以下成品库存：

| 商品 | BOM 规格 | 目标库存 |
| --- | --- | ---: |
| 初晓 | 454g袋装 | 1 袋 |
| 初晓 | 227g袋装 | 4 袋 |
| 曜石2.0 | 454g袋装 | 1 袋 |
| 曜石2.0 | 227g袋装 | 4 袋 |

保留生产批次和生产事件历史；数量调整只更新可用余额，并同步成品汇总、批次分配、库存流水和操作日志。

## 测试证据

先添加失败用例并确认 RED：

- 服务与 HTTP 测试原先拒绝 `spec_g=0` 的历史规格清零请求，返回“spec_g or bom_spec_id required”。
- PostgreSQL API 测试原先无法按有效 BOM 规格身份对生产批次盘点，返回 `product_bom_spec_not_configured`。
- PR 审查补充用例先确认 RED：旧手工入库批次 `qty_units=10`、`remaining_units=0` 时仍被当成零库存，盘点会把汇总误加到 9；批次锁等待期间汇总行已被锁住；手工调整新批次仍未写入可用余额。
- 最新 PR 审查补充用例再确认 RED：重复调用旧人工库存调整入口会因来源唯一键都为 `manual_adjustment/0` 失败；与旧入口并发时，新盘点未等待人工增加批次提交，存在按旧批次快照覆盖汇总的风险。
- 汇总仅有 1 袋 + 20g、没有任何可用批次时，目标调整为 3 袋的用例先确认 RED：接口错误地成功并只建立差额批次。

修复后验证 GREEN：

- `ORDERAPP_TEST_DATABASE_URL=postgres://yiiiple-work@127.0.0.1:5432/postgres?sslmode=disable go test ./internal/infrastructure/postgres/stock ./internal/interfaces/http/stock -run 'Test(FinishedProductAdjustment(ReconcilesBatchesSummaryAndAudit|BatchAdjustmentRollsBackWhenAuditFails|LegacyZeroSpecRequiresExistingBalance)|StandardFinishedBatchAdjustmentAPIReconcilesWarehouseInventory|CanonicalBOMBatchAdjustmentAPIUsesActualProductSpecifications|BOMSpecFinishedTransferAndAdjustmentResolveCurrentVariantPostgresAPI|BOMSpecFinishedInventoryAdjustmentAndTransferKeepCanonicalIdentity)' -count=1`：通过。
- `ORDERAPP_TEST_DATABASE_URL=postgres://yiiiple-work@127.0.0.1:5432/postgres?sslmode=disable go test ./internal/infrastructure/postgres/stock ./internal/interfaces/http/stock -run 'Test(ManualFinishedInventoryAdjustmentsCreateUsableStockBatches|ManualFinishedInventoryIncreaseSerializesWithBatchAdjustment|FinishedAdjustmentRejectsSummaryOnlyFinishedInventory|FinishedAdjustmentRejectsUnallocatedManualReductionBatch|FinishedAdjustmentRejectsLegacyQuantityOnlyManualBatch|FinishedAdjustmentLocksBatchBeforeInventorySummary|FinishedInventorySupportsWarehousesAndFinishedTransfers|FinishedProductAdjustment(ReconcilesBatchesSummaryAndAudit|BatchAdjustmentRollsBackWhenAuditFails|LegacyZeroSpecRequiresExistingBalance)|StandardFinishedBatchAdjustmentAPIReconcilesWarehouseInventory|CanonicalBOMBatchAdjustmentAPIUsesActualProductSpecifications|BOMSpecFinishedTransferAndAdjustmentResolveCurrentVariantPostgresAPI|BOMSpecFinishedInventoryAdjustmentAndTransferKeepCanonicalIdentity)' -count=1`：全部通过。
- `env -u ORDERAPP_TEST_DATABASE_URL -u DATABASE_URL go test ./...`：全部通过。
- `node --test src/lib/operation-manuals.test.js`：3 项通过。
- `./scripts/verify_kferp.sh changed`：通过。
- `git diff --check`：通过。

测试覆盖批次 FIFO 扣减、增加时创建调整批次、规格 0 只允许将已有余额清零、同事务回滚、调整单/批次分配/库存流水/操作日志一致、拒绝未修复的历史数量批次、未分配人工减库存批次和只有汇总没有可用批次的库存、重复人工库存调整可创建多个批次、人工入库与批次盘点以商品规格身份锁串行化，以及调整与出库一致的“先批次、后汇总”锁顺序；正式 HTTP API 使用当前 BOM 规格身份。前两次完整生产预检均在后续审查发现需修复的边界问题时中止；最终预检待本轮补丁提交后执行。

## 生产基线（只读核对）

生产仓库为“吴国宏仓库”，仓库身份 `virtual_5ce21badb5a1326aa295cc83`，客户 ID `488`。只读核对确认仓内没有其他非零成品或物料库存。现有有效 BOM 规格及余额如下：

| 商品 ID | 商品 | BOM 规格 ID | 规格名称 | 当前 | 目标 | 来源批次 |
| ---: | --- | ---: | --- | ---: | ---: | --- |
| 7 | 初晓 | 20 | 454g袋装 | 14 袋 | 1 袋 | FP-OP-0000000044 |
| 7 | 初晓 | 19 | 227g袋装 | 14 袋 | 4 袋 | FP-OP-0000000045 |
| 58 | 曜石2.0 | 69 | 454g袋装 | 1 袋 | 1 袋 | FP-OP-0000000047 |
| 58 | 曜石2.0 | 68 | 227g袋装 | 4 袋 | 4 袋 | FP-OP-0000000046 |

上述 BOM 规格当前均有效并已发布。它们的底层 `spec_g` 字段为 `0`，但 `bom_spec_id` 是有效规格身份；不能按无 BOM 身份的历史 `0g` 规格清除。生产库没有找到需要单独清零的 `bom_spec_id=0` 遗留库存，因此不会对不存在的遗留记录提交调整。

生产环境 BasicAuth 口令已轮换；后续验证不在日志、命令输出或本文件中暴露口令。

## 发布与库存调整结果

待完成：正式生产部署、发布后健康检查、通过 `/api/stock/adjustments` 对实际需要更正的规格提交“吴国宏仓库库存更正”，并只读复核四条目标余额、所有批次余额、库存流水和操作日志。

部署提交、调整单 ID、库存流水 ID、操作日志证据和最终只读核对结果将在生产操作完成后补记。
