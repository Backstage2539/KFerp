# 2026-10-26 生产接口回归的既有失败记录

## 范围与结论

- 对照基线：`main` 的 `001c65b05485cca6ad5b9e241309bb1326039d04`。
- 候选全包回归时：`ebe0542b1640a018125a9024b66b74dddbf8e055` 加生产计划备料分组、运费确认上下文、订单复制字段等未提交整合变更。
- 测试仅使用 `postgres:///kferp_release_20261026?host=/tmp`，每项数据库测试建立自己的 schema；没有操作生产业务数据。
- 修复无包装规格重量入库前，全包结果为 **216 个顶层测试通过、19 个失败、5 个跳过**；包含子测试时为 262 通过、19 失败、5 跳过。包整体失败，不能表述为全包通过。
- 19 项失败都在 main 源码对照中再次失败。随后发现其中无包装规格重量入库是实际账簿一致性缺陷，作为本次发布阻断单独修复；下面记录剩余 **18 项既有失败**。不能据此推断所有旧测试都没有业务风险。

## 对照方式及证据

没有再运行 main 整个包。使用 Go `-overlay` 将候选相对基线修改的已有 Go 文件映射到 `git show 001c65b0:<path>` 的临时副本，分两次只运行 17 项及剩余 2 项失败测试。上述失败测试原文件与 main 一致；新增候选测试未被选入这两次重跑。此证据是 main 源码定向对照，并非另一个完整干净 main checkout 的全量门禁。

本机日志保存在 `/tmp/kferp-production-followup-evidence/`：

- `http-production-full.jsonl`、`http-production-full-summary.json`：候选全包原始结果。
- `main-source-overlay.json`、`http-production-failures-to-recheck.json`：映射及第一批选定失败名称。
- `http-production-failed-main-overlay.jsonl`：17 项在 main 源码上同样失败。
- `http-production-failed-main-overlay-final2.jsonl`：其余 2 项在 main 源码上同样失败。
- `http-production-failed-two-main-debug.log`：只对临时测试副本添加观测，保留原断言；记录零数量入库单和缺 `sku_id` 的订单冻结快照。

上述关键日志已复制到本次发布的独立证据目录 `/Users/yiiiple-work/.codex/worktrees/production-followup-release/evidence/logs/`；原始 `/tmp` 文件继续保留。它们是本次本机证据，不是已提交日志，不保证跨机器保留。

复现候选全包命令（在 `orderapp-remote`，约数分钟）：

```sh
env -u ORDERAPP_RELEASE_BROWSER_READY -u ORDERAPP_RELEASE_PACK_BROWSER_READY -u ORDERAPP_RELEASE_BOM_BROWSER_READY \
  ORDERAPP_TEST_DATABASE_URL='postgres:///kferp_release_20261026?host=/tmp' GOMAXPROCS=2 \
  go test -p 1 -count=1 -json ./internal/interfaces/http/production
```

复现单项 main 源码对照（保留上述临时映射时）：

```sh
ORDERAPP_TEST_DATABASE_URL='postgres:///kferp_release_20261026?host=/tmp' GOMAXPROCS=2 \
  go test -overlay /tmp/kferp-production-followup-evidence/main-source-overlay.json \
  -p 1 -count=1 -v ./internal/interfaces/http/production \
  -run '^TestProductionPlanKeepsFrozenArchivedBOMVariantAfterSameBOMVersionPublish$'
```

后续源码修改后，必须重新生成映射才能声称所有已有源码都来自该基线；不能直接复用旧映射证明新候选的完整基线。

## 剩余 18 项失败及当前归因

### 1. 身份迁移旧 fixture 缺少权威视图初始化：2 项

- `TestProductionPlanKeepsFrozenArchivedBOMVariantAfterSameBOMVersionPublish`，`bom_spec_archived_variant_api_test.go:20`。
- `TestProductionPlanUsesBomSpecIdentityWithoutSalesUnitConversion`，`bom_spec_identity_api_test.go:24`。

两者均在准备测试数据时返回 `SQLSTATE 42P01: product_bom_spec_authorities does not exist`，尚未执行目标断言。它们额外启用迁移 schema，却没有按现行启动依赖准备权威视图。后续应按真实启动顺序修正 fixture 并保留身份断言，不能用 skip 或放宽身份校验处理。

### 2. 旧 BOM / 路线准备及旧响应预期：7 项

- `TestProducePlanSummaryAPIIncludesRoastRowsAndMaterials`。
- `TestProducePlanSummaryAPILeavesAddOnOrdersSelectableWhenOlderOrdersPlanned`。
- `TestProducePlanSummaryAPIReturnsCompatibilityYieldOneForRoastPlans`。
- `TestProducePlanSummaryAPIPreviewsNoLossParentBomMaterialsWhenRouteMissing`。
- `TestProducePlanSummaryAPIDoesNotReplaceInvalidFormalBomWithAnotherRecipe`。
- `TestProducePlanSummaryAPIDripBagBoxCreatesDripDemandAndUpstreamShortage`。
- `TestProducePlanSummaryKeepsValidDemandWhenAnotherOrderHasInvalidInventoryConversion`。

日志分别显示缺少正式商品 BOM、最新 BOM 未配置工艺路线、或旧测试期待的 warning 数组为空。相应入口已经返回不可选择需求和明确 `blocking_reason`；不应为了旧测试恢复用另一配方替代无效正式 BOM，或绕过正式路线保护。后续需分别补齐当前合法 fixture、更新拒绝响应断言，保留换算错误与有效需求相互隔离的业务目标。

### 3. 执行人配置位置及校验顺序已变化：3 项

- `TestProductionPlanRepositoryCreatesSubmitsAndStartsFormalLifecycle`：预期“工单未开始”，实际先返回“本任务尚未分配执行人”。
- `TestJobCardStartUsesWorkOrderIdentityAndAutoDefaultLead`：旧工序负责人写入口返回“工序人员配置已迁移到工位”。
- `TestPR655ScheduleStaffPostgresLifecycle`：同一迁移提示。

后续使用工位人员配置准备执行条件；校验顺序断言应与当前契约一致，不应恢复已退役的工序配置入口。

### 4. 计划供给 / 仓库准备不满足提交校验：4 项

- `TestHistoricalWorkOrderStartUsesReservationRequirementsWhenMaterialSnapshotIsMissing`：提交时缺少计划生豆 600g。
- `TestReleasedWorkOrderIssueMakesWIPReadyThenStarts`：提交时同样缺少 600g。
- `TestProductionPlanAPIKeepsSameSKUWithDifferentFrozenParentsIsolated`：提交时旧父商品原料缺少 505g。
- `TestTypedProductDependencyConsumesBatchesFromFrozenWarehouses`：旧 `picking_version=0` 分支提交时，所选 `wip` 来源缺少中间品 10000g。

这些失败都在候选和 main 上发生于准备后续场景的提交阶段。当前只证明主干也拒绝该供给状态，尚未通过修正 fixture 完成原目标场景，因此不能统称为无业务影响。应另项核对当前供给选择及转仓契约，再补齐 fixture；不能删除真实库存不足校验。

### 5. 订单冻结快照缺少 SKU 身份：1 项

- `TestProductionPlanCreatesDirectProductUnitDependencyChain`。

`product_identity_single_output_plan_api_test.go` 构造的 `production_quantity_snapshot` 含父商品和换算，但没有 `sku_id`。`unprod_summary.go` 明确要求冻结快照的 `sku_id` 与具体商品匹配，实测响应为“订单冻结的生产数量换算与具体 SKU 不一致或不完整”。后续应修正该快照准备，再验证真实计件依赖链，不应绕过身份保护。

### 6. 已支持计件生产，而旧测试仍期待拒绝：1 项

- `TestProductionPlanAPIInheritsParentBOMAndFreezesSalesSpecConversion`。

测试中计件商品已配置正式 BOM、路线及每件 1 个组件；API 返回包含该物料快照的 2 件草稿，旧断言仍要求拒绝“空物料计划”。返回内容并非空物料计划。后续应按当前计件生产契约验证物料、数量和父商品来源，另保留真正缺料配置的拒绝用例。

## 单独修复的无包装规格重量入库缺陷

`TestRecordedWeightOutputWithoutPackageSpecReceiptsAsGrams` 不是 fixture 失败：main 在 `spec_g=0`、实际产出 5.6kg 时增加库存和库存台账 5600g，但入库明细写入 0g。旧 `FinishedTotalGrams` 对 `spec_g<=0` 返回 0，工序入库路径将它传给入库单、批次和生产日志。该问题属于既有业务逻辑缺陷，与本次待验收的入库路径相关，故作为发布阻断处理。

本次最小修复仅在 `receiveOperationOutputTx` 中使用实际重量单位换算得到的克数；不改变全局 `FinishedTotalGrams` 语义、不修改 schema、不修复历史记录。强化测试包含 kg/g、已耗料成本 504 加工序成本 56、库存/批次/台账/单据/生产日志 5600g、单位成本 100/kg、总成本 560，以及重复提交不重复入库/扣料。RED/GREEN 分别见 `unbagged-weight-receipt-red.log`、`unbagged-weight-receipt-green.log`。

相关回归 `TestRecorded|TestLegacyReview|TestOperationCompletion|TestRoastingPartialCompletion` 共 **21 个顶层测试、39 个含子测试全部通过，0 失败、0 跳过**，见 `unbagged-weight-receipt-related.jsonl`。这不是修复后重新执行整个 production HTTP 包的结果；全包原始失败日志继续保留。

该最小修复及测试随后由发布负责人提交为 `bbdaa13465858be3a002bf1149dc625938c461d3`。

发布负责人已对生产做只读影响核查，证据 `/Users/yiiiple-work/.codex/worktrees/production-followup-release/evidence/production-bulk-receipt-impact.json` 返回 `weight_receipts_with_zero_details=0`、`since_previous_release=0`（另记录 `completed_event_output_g=0`）。这表示该核查口径下未找到受影响记录，不是对所有历史数量场景的泛化证明；本次没有修复、重算或写入生产历史数据。

## 跳过边界与未修范围

5 个跳过均为需要显式启用、持续运行供浏览器连接的 fixture：`TestPR659LogsBrowserFixture`、`TestPR658BrowserFixture`、`TestProductionReleaseBrowserFixture`、`TestProductionReleasePackagingBrowserFixture`、`TestProductionReleaseBOMReturnBrowserFixture`。正常全包运行没有启用这些服务；跳过不等于已经完成浏览器验收。新增发布 harness 只在 `_test.go`，必须显式 READY 环境变量、限定专用本机数据库，且只监听 `127.0.0.1`。

本记录不宣称生产部署已完成，不修复上述 18 项测试，不改现有需求验收状态，也不覆盖默认 BOM 实际规格切换的并发风险。后者独立记录在 `2026-10-26-existing-bom-switch-race.md`。
