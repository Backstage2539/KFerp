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
- 最新 PR 审查回归也先确认 RED：生产模块二次迁移把 `manual_adjustment` 排除条件删掉，第二笔同身份手工调整批次报唯一键冲突；普通成品转仓未先取规格身份锁，调整持有批次锁时转仓先锁汇总行。
- 后续审查回归先确认 RED：完工入库与盘点没有共用规格锁；非 BOM 成品转仓只改了汇总，源批次仍为 10、目标仓没有批次；普通非 BOM 转仓也被规格解析错误地拒绝。
- 最新审查回归先确认 RED：跨多来源批次的非 BOM 转仓触发 `stock_batches_source_uq`；仅有质检冻结批次时误建期初批次；散装克重跨规格单位边界后汇总显示成袋数但批次仍只有克重。
- 最新审查补充回归先确认 RED：无批次移动追踪的历史非 BOM 转仓撤销误按新分类处理；服务层与入账层会将 `0 袋 + 1000g` 的散装转仓归一化为 1 袋。
- 最新锁顺序回归先确认 RED：销售出库没有取得转仓/盘点共用的规格身份锁，转仓与出库可以采用相反的汇总行/批次行锁顺序。
- 最新审查回归先确认 RED：散装转仓后又有完工入库时，汇总克重归一为整袋，撤销旧转仓仍按原始散克维度相减，产生负散克库存。

修复后验证 GREEN：

- `ORDERAPP_TEST_DATABASE_URL=postgres://yiiiple-work@127.0.0.1:5432/postgres?sslmode=disable go test ./internal/infrastructure/postgres/stock ./internal/interfaces/http/stock -run 'Test(FinishedProductAdjustment(ReconcilesBatchesSummaryAndAudit|BatchAdjustmentRollsBackWhenAuditFails|LegacyZeroSpecRequiresExistingBalance)|StandardFinishedBatchAdjustmentAPIReconcilesWarehouseInventory|CanonicalBOMBatchAdjustmentAPIUsesActualProductSpecifications|BOMSpecFinishedTransferAndAdjustmentResolveCurrentVariantPostgresAPI|BOMSpecFinishedInventoryAdjustmentAndTransferKeepCanonicalIdentity)' -count=1`：通过。
- `ORDERAPP_TEST_DATABASE_URL=postgres://yiiiple-work@127.0.0.1:5432/postgres?sslmode=disable go test ./internal/infrastructure/postgres/stock ./internal/interfaces/http/stock -run 'Test(ManualFinishedInventoryAdjustmentsCreateUsableStockBatches|ManualFinishedInventoryIncreaseSerializesWithBatchAdjustment|FinishedAdjustmentRejectsSummaryOnlyFinishedInventory|FinishedAdjustmentRejectsUnallocatedManualReductionBatch|FinishedAdjustmentRejectsLegacyQuantityOnlyManualBatch|FinishedAdjustmentLocksBatchBeforeInventorySummary|FinishedInventorySupportsWarehousesAndFinishedTransfers|FinishedProductAdjustment(ReconcilesBatchesSummaryAndAudit|BatchAdjustmentRollsBackWhenAuditFails|LegacyZeroSpecRequiresExistingBalance)|StandardFinishedBatchAdjustmentAPIReconcilesWarehouseInventory|CanonicalBOMBatchAdjustmentAPIUsesActualProductSpecifications|BOMSpecFinishedTransferAndAdjustmentResolveCurrentVariantPostgresAPI|BOMSpecFinishedInventoryAdjustmentAndTransferKeepCanonicalIdentity)' -count=1`：全部通过。
- `env -u ORDERAPP_TEST_DATABASE_URL -u DATABASE_URL go test ./...`：全部通过。
- `node --test src/lib/operation-manuals.test.js`：3 项通过。
- `./scripts/verify_kferp.sh changed`：通过。
- `git diff --check`：通过。
- 新增迁移顺序回归 `TestStockThenProductionLedgerMigrationsKeepManualAdjustmentSourcesRepeatable` 与并发转仓回归 `TestCanonicalFinishedTransferSerializesWithBatchAdjustment`：修复前分别因 `stock_batches_source_uq` 唯一键冲突、转仓未等待规格身份锁而失败；两项修复后均通过。
- 新增完工并发回归 `TestFinishedManufactureReceiptSerializesWithBatchAdjustment`：移除完工入库规格锁后先 RED（完工入库未等待调整身份锁），恢复共享锁后 GREEN，核对调整后的汇总与新增生产批次余额一致。
- 新增非 BOM 转仓回归 `TestLegacyFinishedTransferMovesFIFOStockBatches`：覆盖跨两个来源批次的 FIFO 转仓、每次批次移动的唯一来源身份、库存流水和撤销恢复。
- 新增 `TestLegacyFinishedTransferDoesNotCreateOpeningBatchForFrozenStock` 与 `TestLegacyFinishedTransferKeepsLooseGramsAcrossPackageBoundary`：修复后冻结批次阻止转仓且不重复创建期初批次；非 BOM 转仓保留袋数和散装克重分类，跨仓累计克重不触发隐式包装，撤销后仍可恢复批次与汇总。
- 新增 `TestLegacyFinishedTransferCancellationPreservesHistoricalNormalization`、`TestTransferFinishedProductPreservesLooseGramRequest`，并扩展跨规格边界转仓用例：历史转仓撤销沿用既有归一化；满规格重量的散克转仓仍保持克重。修复前 RED，修复后目标测试 GREEN。
- 新增销售 PostgreSQL 回归 `TestSalesShipmentLocksFinishedIdentityBeforeBatchAndSummary`：修复前在转仓持有商品规格锁时销售扣库存仍直接完成；销售出库改为按固定顺序获取本单全部规格锁后测试通过，避免其与转仓反向持有批次/汇总锁。
- 新增 `TestLegacyFinishedTransferCancellationAfterReceiptUsesBatchBalances`：750g 散克转仓后再完工入库 250g，汇总先显示为 1 袋；修复前撤销写入 `1 袋 - 750g`，按批次移动记录重算后恢复为来源仓 750g、目标仓仅留 250g。
- 最新 PR 审查增加源仓转仓回归 `TestLegacyFinishedTransferAfterSourceReceiptUsesBatchBalances` 与撤销回归 `TestLegacyFinishedTransferCancellationAfterSourceReceiptUsesBatchBalances`。在隔离 PostgreSQL 上先 RED：完工入库把 750g+250g 汇总成 1 袋后再转出 250g，旧代码留下 `1 袋 -250g`；转仓后在来源仓完工入库并撤销，旧代码错误写成 `1 袋 +1250g`。采用来源/目标批次余额重建非 BOM 摘要、修正撤销变量遮蔽后两项 GREEN。
- 最新销售审查增加 `TestSourceWarehouseShipmentDeductsBatchAndSummaryTogether`。先 RED：客户虚拟仓发货后摘要为 `0+500g`，但原批次仍是 `1/1500g`，也没有批次扣减记录；按订单来源仓读取 FIFO 批次并同步摘要/流水后 GREEN。`TestSalesShipmentLocksFinishedIdentityBeforeBatchAndSummary` 仍通过。
- 新一轮 PR 审查补充两个回归并先 RED：目标仓原本只有汇总库存时，旧转仓取消将目标仓清零；其他仓订单的预留量错误占用当前仓汇总库存，导致当前仓发货报库存不足。新增目标仓期初批次追溯并按订单来源仓过滤预留后，两项定向 PostgreSQL 用例 GREEN。
- 新一轮审查还指出同一事务批量填多个订单物流时，逐单获取锁可能因订单顺序相反而死锁。新增 `TestMultiOrderShipmentIdentityLocksAreOrderIndependent`，覆盖反序订单集合在同一已排序身份锁集合上的串行等待；`FillShipmentTracking` 与 `FillShipmentTrackingByOrderNo` 现在会在开始更新前预锁整批订单的库存身份。
- 临时 PostgreSQL 全量运行 `internal/infrastructure/postgres/sales` 时，销售包内其他既有测试因其共享 `EnsureSchema` fixture 缺少 `orders.is_void`、`orders.ship_status_id` 等列及一条 canonical order item 未找到而失败；本次改动对应的来源仓出库、跨仓预留和批量身份锁定向回归均独立通过。无数据库环境的完整 `go test ./...` 通过。
- 新增 `TestStockDocumentFinishedIdentityLocksUseStableOrder`：两笔多商品库存单按相反商品顺序并发获取库存身份锁后串行完成；提交和撤销在批次或汇总行写入前按商品、BOM 规格、克重稳定排序取得全单身份锁。
- 本次审查修复后，临时本机 PostgreSQL 上新增目标仓期初批次、跨仓预留隔离、批量物流库存锁和非默认来源仓出库回归均通过。前一次 pushed commit `0b0aecfc3` 的生产预检在服务器 Go 全量测试时因磁盘空间不足（`no space left on device`）中止；容器镜像与构建缓存占用较大，预检自动清理后根分区仅余约 210MB。没有清理服务器文件或容器镜像，也没有部署或写入生产库存。最终推送后的复审与生产预检仍待完成；部署前需要先处理磁盘空间阻塞。

测试覆盖批次 FIFO 扣减、增加时创建调整批次、规格 0 只允许将已有余额清零、同事务回滚、调整单/批次分配/库存流水/操作日志一致、拒绝未修复的历史数量批次、未分配人工减库存批次和只有汇总没有可用批次的库存、重复人工库存调整可创建多个批次、人工入库与完工入库并发串行化、规范与非 BOM 转仓 FIFO/撤销/冻结阻断、散装克重分类一致，以及调整与出库一致的“先批次、后汇总”锁顺序；正式 HTTP API 使用当前 BOM 规格身份。生产预检在后续审查发现边界问题时中止过两次；最新一次 `0b0aecfc3` 预检因服务器磁盘空间不足中止，尚未对当前修复后的提交重跑。

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
