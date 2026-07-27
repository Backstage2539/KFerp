# PR-559 生产配置、WIP 提示与工单领料验收

## 范围

- 生产 BOM 并入生产配置第 4 个 Tab，并保留旧链接、指定 BOM、客户上下文与返回来源。
- released 工单在尚无 reservation 时，仍按冻结物料快照计算重量/计数物料的 WIP 覆盖。
- 工单生产领料显示真实工单号，一张库存单据带出全部短缺物料，每行只维护一个数量并自动显示库存单位。
- 合并并部署 development；production 不部署。开发环境指定真实工单只做只读预览，不提交库存、不开始生产。

## RED 证据

- `TestStockFrozenMaterialRequirementsHonorConsumeUnitConversion` 在修复前把 `consume_unit=kg` 的 `1` 解释为 `1g`，而不是权威换算后的 `1000g`。
- `TestUnifiedStockDocumentListAndDetailExposeWorkOrderNumberAndCountTotal` 在修复前查询不存在的 `d.work_order_no`，真实 PostgreSQL 返回 `column d.work_order_no does not exist`。
- `TestCreateStockDocumentRejectsNonManufacturingPurposeBoundToWorkOrder` 在修复前允许原料入库或普通转仓携带工单关联。
- `TestHistoricalWorkOrderStartUsesReservationRequirementsWhenMaterialSnapshotIsMissing` 在修复前回退实时 BOM，错误要求后来配置的“计划生豆 600g”，与页面展示的历史 reservation 不一致。
- `TestHistoricalWorkOrderIssueUsesReservationRequirementAndCurrentWIPShortage` 在修复前允许只有历史 reservation 的工单超过当前 WIP 缺口领料。
- `TestWorkOrderStockDocumentRejectsPurposeItemTypeMismatch` 在修复前允许绑定工单的生产领料混入成品行、完工入库混入物料行。
- 新增的 released/no-reservation、重量/计数物料、历史 reservation 与多物料领料契约在实现前缺少统一覆盖字段和行为，不能通过编译或断言。

## GREEN 证据

- 完整 Go：`cd orderapp-remote && go test ./... -count=1`，全部通过。
- 仓库 changed verifier：`scripts/verify_kferp.sh changed`，通过。
- 前端定向：
  - `production-execution-hub.test.js`、`menu-ia.test.js`、`production-system-menu-consolidation.test.js` 中 PR-559 相关断言全部通过。
  - `scripts/verify_kferp.sh frontend-build`，Vite 构建通过。
- 临时 PostgreSQL：
  - `TestWorkOrderWIPCoverageSupportsWeightCountAndOtherReservations`
  - `TestHistoricalWorkOrderWIPCoverageFallsBackToReservationRequirementsAndIgnoresClosedOrders`
  - `TestStockFrozenMaterialRequirementsHonorConsumeUnitConversion`
  - `TestUnifiedStockDocumentListAndDetailExposeWorkOrderNumberAndCountTotal`
  - `TestReleasedWorkOrderIssueUsesFrozenSnapshotWithoutReservationAndGuardsQuantity`
  - `TestHistoricalWorkOrderStartUsesReservationRequirementsWhenMaterialSnapshotIsMissing`
  - `TestHistoricalWorkOrderIssueUsesReservationRequirementAndCurrentWIPShortage`
  - `TestWorkOrderStockDocumentRejectsPurposeItemTypeMismatch`
  - `TestReleasedWorkOrderIssueMakesWIPReadyThenStarts`
  - 上述测试全部通过；最后一项覆盖 `released 工单 -> 原料仓入库 -> 一个 SE 多物料生产领料 -> WIP 就绪 -> 开工`。
- 完整前端测试为 `812/819`。7项失败逐文件在未改动的 `origin/develop` 基线复现，属于既有客户工作区、客户档案入口和视图上下文静态断言；PR-559 没有新增失败。
- 带外部 PostgreSQL 的全仓测试仍有合同 `audit_logs` 测试 schema 和客户 `contact NOT NULL` 两项既有失败；相同命令已在干净 `origin/develop` 基线复现。PR-559 的定向真实 PostgreSQL 门禁全部通过。

## 手册

- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_STOCK.md`
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`

## 开发环境只读冒烟

- 指定工单：`WO-PP-0000000080-0000000050`
- 验证内容：真实工单号、逐物料 WIP 缺口、生产领料多物料预览、单数量与库存单位。
- 禁止动作：不保存/提交库存单据，不开始生产，不修改 BOM、工单、库存或生产日志。

## 结论

- 代码与自动化验证已完成；等待合并、development 部署和指定工单只读冒烟。production 明确不部署。
