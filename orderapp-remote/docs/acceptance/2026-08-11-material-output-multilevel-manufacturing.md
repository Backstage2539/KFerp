# PR-598 从物料产出到多层生产：验收记录

## 范围与冻结合同

- `is_semi_finished` 只是物料的业务标识、筛选和展示属性；它不限制物料成为 BOM 产出，也不参与排产授权。
- `can_manufacture` 只由该物料是否存在默认且已发布的产出 BOM 计算；勾选或取消“是否半成品”都不授予或撤销制造能力。
- 普通生产 BOM 使用 `output_type=product|material` 和对应产出对象。任意有效物料可被生产，旧商品 BOM 与历史快照保持兼容。
- 生产计划按默认已发布 BOM 递归展开，先做库存覆盖，再按净缺口生成上游供应和工单依赖；循环、缺默认 BOM 或单位不兼容必须阻断。
- 本记录覆盖自动化交付证据，不代表 Van 已完成页面或真实业务验收。

## RED 证据

- 前端定向命令：`node --test src/lib/materials-ui.test.js src/lib/bom.test.js src/lib/produce-plan.test.js src/lib/work-orders.test.js src/lib/production-execution-hub.test.js src/lib/operation-manuals.test.js src/lib/menu-ia.test.js`。
- 初始结果：142 项中 130 通过、12 失败。失败点覆盖物料半成品 / 可制造显示、物料 BOM 往返、typed BOM 产出、递归计划图、工单与执行上游阻断、库存作业手册入口。
- API 对齐补充 RED：production 当前只返回持久化的 typed `items[]` 与 `supply_gaps[]`，尚不返回 `manufacturing_plan`；新增 fallback 测试后 `produce-plan.test.js` 51 / 53，通过项不受影响，新增两项因 fallback 和计划详情渲染缺失而失败。
- 目标仓与取消动作补充 RED：`produce-plan.test.js + work-orders.test.js + production-execution-hub.test.js` 为 81 / 86 通过，5 项分别因目标仓 API / 冻结 UI、已提交计划取消、关联工单类型化产出、工单列表取消和执行枢纽取消缺失而失败。物料半成品筛选单测为 17 / 18 通过，新增筛选合同失败。
- 真实 flat graph 回归 RED：按后端 `{key, output_type, output_product_id, output_material_id, output_name, ...}` 节点结构增加回归后，`produce-plan.test.js` 为 56 / 57 通过；商品节点退化为 `product:0`，三个物料节点碰撞成一条 `material:0`。
- 支持合同命令：`go test ./internal/interfaces/http/support -run TestDev598MaterialOutputMultilevelManufacturingContracts -count=1`。
- 初始结果：失败，`req_store.go` 缺少 PR-598 产品 / 开发 / 审核种子，且需求、验收与四本手册尚未包含冻结合同。

## GREEN 证据

- 前端定向：同一组 7 个测试文件最终 143 / 143 通过；其中新增保护确认后端 `has_unfinished_dependencies`、`dependency_blocking_reason`、`upstream_work_order_ids` 即使与 readiness 暂时不同步，也会禁用开始生产并显示上游阻断原因。最终 `scripts/verify_kferp.sh all` 通过 `find` 发现全部测试文件，frontend find 全量 983 / 983 通过，0 失败。
- API 对齐补充 GREEN：`produce-plan.test.js` 53 / 53 通过；没有 `manufacturing_plan` 时，生产计划详情可直接用当前 `items[] + supply_gaps[]` 展示 typed 产出、层级、库存覆盖字段、净缺口、补足方式与上游阻断，后端未来补 graph JSON 时继续优先使用 graph。补充后 Vite build 与 `git diff --check` 再次通过。
- 目标仓、取消与物料筛选补充 GREEN：上述 3 个生产前端测试加 `materials-ui.test.js` 为 104 / 104 通过。草稿计划行调用 PATCH 保存正式仓库，提交后只读；已提交未开工计划与 released 未开工工单提供带确认的取消并刷新；关联工单显示类型化产出。真实 flat graph 修复后 `produce-plan.test.js` 为 57 / 57 通过，显式 `key` 优先，typed output ID / name 兼容；最终前端全量计数见 983 / 983 复验。
- 支持合同：`go test ./internal/interfaces/http/support -run TestDev598MaterialOutputMultilevelManufacturingContracts -count=1` 通过；随后 support 全包 `go test ./internal/interfaces/http/support -count=1` 通过。
- 全量门禁：`scripts/verify_kferp.sh all` exit 0，Go 全包、frontend find 全量 983 / 983、Vite 2.08s 均通过（仅既有大 chunk 提示）；`git diff --check` 通过。
- production HTTP 真实 PostgreSQL 全包 86.736s：`ORDERAPP_TEST_DATABASE_URL='postgres://127.0.0.1:55432/kferp_test?sslmode=disable' go test ./internal/domain/production ./internal/application/production ./internal/infrastructure/postgres/production ./internal/interfaces/http/production -count=1` 全部通过，四包依次 0.236s / 0.471s / 1.825s / 86.736s。
- BOM / material / catalog / costing / stock 真实 PostgreSQL：`KF_RUN_POSTGRES_INTEGRATION=1 PGHOST=127.0.0.1 PGPORT=55432 PGDATABASE=kferp_test PGSSLMODE=disable ORDERAPP_TEST_DATABASE_URL='postgres://127.0.0.1:55432/kferp_test?sslmode=disable' go test ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/materials ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/infrastructure/postgres/stock -count=1` 全部通过，依次 2.829s / 2.930s / 2.041s / 1.348s / 8.092s。
- 默认切换循环与旧库 repair：`TestPR598DefaultBindingSwitchRejectsTypedGraphCyclesPostgres` 验证切换默认 BOM 仍执行 typed 图循环门禁；`TestRepairLegacyProductionBomBindingsPostgresOnce` 验证旧库绑定仅修复一次，两项均通过。
- direct product complete / partial / multi / cancel：`TestProductWorkOrderCompleteWithoutStockDocumentCreatesOneAtomicReceipt`、`TestProduceRunningPartialFinishAuditFailureRollsBackAllChanges`、`TestProduceFinishAPIMultiSpecFinalAuditFailureRollsBackAllOutputs`、`TestActiveTypedWorkOrderCancelPersistsNoteInBothAtomicAudits` 与 `TestPausedAndPartiallyCompletedCancelKeepsRunningReservationWIPAndDemandConsistent` 均通过。
- 最终审计原子回滚与取消 Note：`TestProductWorkOrderCompleteWithoutStockDocumentRollsBackReceiptAndFinalAuditFailures`、`TestActiveTypedWorkOrderCancelFinalRunningAuditFailureRollsBackReservationsAndPriorAudit` 通过；完工或取消的最终审计失败时，入库、预留、running 状态和先前审计同事务回滚，成功取消时 Note 同时进入两个原子审计。

## 核心自动化验收矩阵

| 合同 | 自动化锚点 | 预期 |
|---|---|---|
| 半成品标识与可制造能力分离 | `materials-ui.test.js`、materials API tests | 标识可独立编辑；能力只读且只随默认已发布 BOM 变化 |
| 商品 / 物料统一 BOM 产出 | `bom.test.js`、BOM service / API tests | typed identity 往返一致；任意有效物料可产出；旧商品兼容 |
| 多层净需求 | `produce-plan.test.js`、multilevel planning tests | `22.7kg - 10kg = 12.7kg`，库存不重复覆盖，共享缺口合并 |
| 工单依赖 | `work-orders.test.js`、`production-execution-hub.test.js`、work-order API tests | 未完成上游显示原因 / 编号并阻止下游开工 |
| 物料完工入库 | stock / production API tests、库存手册 | 合格量进入目标仓库与批次，重复完成不重复入库 |
| 递归成本 | recursive costing tests、成本手册 | 各层 BOM 成本汇总，循环阻断，历史快照不回算 |
| Vue 手册入口与追踪 | operation-manual / menu / support tests | 五页语义一致，库存作业手册可见，PR/DEV/REV 可追踪 |
| 目标仓与未开工取消 | `produce-plan.test.js`、`work-orders.test.js`、`production-execution-hub.test.js` | 草稿可保存目标仓并在提交后冻结；未开工计划 / 工单可安全取消并刷新；关联工单显示 typed 产出 |

## Van 业务验收

- 浏览器人工验收未执行；页面检查和 Van 的真实业务验收明确不属于本轮自动验收。
- 建议后续在 development 依次核对：物料标识 → 建立物料产出 BOM → 发布并设默认 → 商品计划查看递归净缺口 → 提交依赖工单 → 上游物料入库 → 下游解锁。
- `REV-598-MATERIAL-OUTPUT-MULTILEVEL-MANUFACTURING` 保持 `todo`，由 Van 在 development 完成业务验收后关闭。

## 交付边界

- 合并与 development 部署均未执行，状态保持 pending。
- production 环境不在自动验收范围内，未部署、未写入、未做业务验证。
- 本工作未提交、未推送、未执行浏览器或人工业务验收。
