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
- 最新 PR 审查补充的来源仓范围回归先确认 RED：来源仓只有汇总余额、另一仓已有批次时，来源仓被错误判断为“已有批次但不可用”而拒绝转仓。
- 最新 PR 审查补充的订单锁顺序回归先确认 RED：Excel 批量物流更新先取库存身份锁、后取订单行锁，与单订单物流更新反序，可能死锁。
- 新一轮审查回归先确认 RED：转仓后手工扣减目标仓汇总库存，旧撤销流程仍按未扣减的批次余额恢复，导致库存被重新增加；库存单提交/撤销也可能先取成品规格锁、再等待工单行，与新建并提交反序死锁。

修复后验证 GREEN：

- `ORDERAPP_TEST_DATABASE_URL=postgres://yiiiple-work@127.0.0.1:5432/postgres?sslmode=disable go test ./internal/infrastructure/postgres/stock ./internal/interfaces/http/stock -run 'Test(FinishedProductAdjustment(ReconcilesBatchesSummaryAndAudit|BatchAdjustmentRollsBackWhenAuditFails|LegacyZeroSpecRequiresExistingBalance)|StandardFinishedBatchAdjustmentAPIReconcilesWarehouseInventory|CanonicalBOMBatchAdjustmentAPIUsesActualProductSpecifications|BOMSpecFinishedTransferAndAdjustmentResolveCurrentVariantPostgresAPI|BOMSpecFinishedInventoryAdjustmentAndTransferKeepCanonicalIdentity)' -count=1`：通过。
- `ORDERAPP_TEST_DATABASE_URL=postgres://yiiiple-work@127.0.0.1:5432/postgres?sslmode=disable go test ./internal/infrastructure/postgres/stock ./internal/interfaces/http/stock -run 'Test(ManualFinishedInventoryAdjustmentsCreateUsableStockBatches|ManualFinishedInventoryIncreaseSerializesWithBatchAdjustment|FinishedAdjustmentRejectsSummaryOnlyFinishedInventory|FinishedAdjustmentRejectsUnallocatedManualReductionBatch|FinishedAdjustmentRejectsLegacyQuantityOnlyManualBatch|FinishedAdjustmentLocksBatchBeforeInventorySummary|FinishedInventorySupportsWarehousesAndFinishedTransfers|FinishedProductAdjustment(ReconcilesBatchesSummaryAndAudit|BatchAdjustmentRollsBackWhenAuditFails|LegacyZeroSpecRequiresExistingBalance)|StandardFinishedBatchAdjustmentAPIReconcilesWarehouseInventory|CanonicalBOMBatchAdjustmentAPIUsesActualProductSpecifications|BOMSpecFinishedTransferAndAdjustmentResolveCurrentVariantPostgresAPI|BOMSpecFinishedInventoryAdjustmentAndTransferKeepCanonicalIdentity)' -count=1`：全部通过。
- `env -u ORDERAPP_TEST_DATABASE_URL -u DATABASE_URL go test ./...`：全部通过。
- `node --test orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.test.js`：3 项通过。
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
- 新增 `TestLegacyFinishedTransferSourceOpeningBalanceIsWarehouseScoped`：修复前，来源仓只有汇总库存而 `finished_goods` 仓有独立批次时，转仓错误返回“finished product stock is blocked”；按来源仓与最新库存流水筛选批次后，测试验证来源/目标汇总与批次余额均为 3 袋。
- 新增 `TestShipmentTrackingPrelocksOrdersBeforeFinishedIdentities`：先让单订单路径持有订单行与库存身份锁，再启动反序订单集合的批量锁定；修复后测试确认批量路径在等待订单行时尚未申请库存身份锁，释放后整批成功。
- 新增 `TestLegacyFinishedTransferCancellationRejectsManualSummaryBatchMismatch`：先转出 1 袋，再模拟旧手工盘点只改汇总余额为 0；修复前取消成功并恢复了已扣库存，RED 证据为 `cancel transfer ... = <nil>`。取消前校验两个仓的批次总克重与汇总克重一致后，定向 PostgreSQL 测试通过且原摘要/批次仍为来源 1/1、目标 0/1。
- 新增 `TestStockDocumentPrelocksWorkOrderBeforeFinishedIdentity`：新回归最初因缺少工单优先锁辅助函数而无法编译；补上统一锁顺序后，测试确认第二笔事务先等待工单锁、尚未申请规格 advisory lock，释放后成功完成。
- 最新两项审查修复完成后，`env -u ORDERAPP_TEST_DATABASE_URL -u DATABASE_URL go test ./...`、临时 PostgreSQL `go test ./internal/infrastructure/postgres/stock -count=1`、操作手册测试 3 项、`./scripts/verify_kferp.sh changed` 均通过。
- 临时 PostgreSQL 全量运行 `internal/infrastructure/postgres/sales` 时，销售包内其他既有测试因其共享 `EnsureSchema` fixture 缺少 `orders.is_void`、`orders.ship_status_id` 等列及一条 canonical order item 未找到而失败；本次改动对应的来源仓出库、跨仓预留和批量身份锁定向回归均独立通过。无数据库环境的完整 `go test ./...` 通过。
- 新增 `TestStockDocumentFinishedIdentityLocksUseStableOrder`：两笔多商品库存单按相反商品顺序并发获取库存身份锁后串行完成；提交和撤销在批次或汇总行写入前按商品、BOM 规格、克重稳定排序取得全单身份锁。
- 本轮审查修复后，临时本机 PostgreSQL 上完整库存包、来源仓期初批次、跨仓预留隔离、批量物流订单先锁和非默认来源仓出库回归均通过；无数据库环境的完整 Go suite 与 changed verifier 通过。此前销售 PostgreSQL 全包测试仍有共享 schema fixture 缺列与 canonical order item 缺失问题，相关新增定向用例通过。

测试覆盖批次 FIFO 扣减、增加时创建调整批次、规格 0 只允许将已有余额清零、同事务回滚、调整单/批次分配/库存流水/操作日志一致、拒绝未修复的历史数量批次、未分配人工减库存批次和只有汇总没有可用批次的库存、重复人工库存调整可创建多个批次、人工入库与完工入库并发串行化、规范与非 BOM 转仓 FIFO/撤销/冻结阻断、散装克重分类一致，以及调整、转仓、销售出库和物流批量更新的一致锁顺序；正式 HTTP API 使用当前 BOM 规格身份。新增的汇总/批次不一致撤销拒绝和工单/库存身份锁顺序两项 review 回归已在本地通过。生产预检已在清理空间后对最终 main 提交通过。

## 生产预检前磁盘清理

- 用户授权删除缓存和未使用的旧镜像后，只读确认生产服务器根分区曾为 100% 使用、仅余约 207MB；Docker 构建缓存 21.25GB，旧回滚标签中 50 个未被任何容器引用。
- 已删除 50 个未使用旧回滚镜像标签，保留生产/开发最新回滚镜像和被退出 PR 验收容器引用的旧开发镜像；执行 `docker builder prune -af` 清理 21.25GB 构建缓存。
- 清理后根分区为 59GB 总量、29GB 已用、28GB 可用（51%）；Docker 镜像 60→11 个，构建缓存 0B。容器、数据卷和数据库未删除，服务未重启。
- 最终生产预检仍待最新修复提交推送、审查完成后重跑；本次清理没有部署应用或写入生产库存。

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

- PR #312 已合入 develop，提交 `7eb69b040cdfcd70ba8283a3f7f994b0ca191457`；生产 PR #311 已合入 main，提交 `d5cd5b66c6bee6854feeca72a6ac1920e0efff9a`。
- 最终生产预检针对 `origin/main@d5cd5b66c6bee6854feeca72a6ac1920e0efff9a` 通过。2026-10-27 10:32（北京时间）使用 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production` 发布，只重建/重启 `erp_prod_orderapp`；生产登录页 HTTP 200，PostgreSQL 与文档转换服务保持运行。未导出或发布微信小程序。
- 部署备份：`/opt/stacks/erp-production/orderapp.backup.deploy-20261027103203-d5cd5b66c6be`；回滚镜像：`kferp-orderapp-rollback:production-20261027103203-d5cd5b66c6be`。
- 正式调整通过 `POST /api/stock/adjustments` 完成：调整单 19 将初晓 BOM 规格 20（454g袋装）从 14 袋调整为 1 袋，FIFO 扣减批次 `FP-OP-0000000044` 13 袋；调整单 20 将初晓 BOM 规格 19（227g袋装）从 14 袋调整为 4 袋，FIFO 扣减批次 `FP-OP-0000000045` 10 袋。两次请求均返回 HTTP 200。
- 曜石2.0 的 BOM 规格 69（454g）1 袋和规格 68（227g）4 袋原本已经符合目标，未提交无变化调整。
- 最终只读 API `GET /api/stock/warehouse-inventory` 对吴国宏仓库返回 `total=4`：初晓 454g×1、227g×4；曜石2.0 454g×1、227g×4。`finished_inventory` 汇总共 4 行、10 袋；按商品、BOM 规格及货主与 `stock_batches` 比对，每行汇总袋数与剩余批次数一致，散装克重均为 0。唯一非零的四个生产批次为 `FP-OP-0000000044/45/46/47`，生产事件历史仍在。
- 调整单 19/20 的库存流水 ID 为 211/212，批次分配分别为 `-13`、`-10` 袋；审计日志 ID 为 8672/8673；操作日志 ID 为 256249/256267，均记录 `POST /api/stock/adjustments` 且 HTTP 200。调整单、流水、FIFO 批次分配和审计记录可相互追溯。

### 调整原因文字差异

两笔提交实际记录的原因均为“吴国宏仓库存更正”，与计划指定文本“吴国宏仓库库存更正”相比少一个“库”字。当前正式接口只支持新建调整，不提供已提交调整单的原因修改入口；我保留了原正式调整记录，没有用直接数据库改写或无库存变化的重复调整掩盖差异。库存数量及完整的批次、流水、审计和 HTTP 操作日志均已核实；原因文字差异留作明确的验收偏差。
