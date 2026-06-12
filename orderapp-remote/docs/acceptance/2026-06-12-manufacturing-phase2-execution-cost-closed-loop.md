# PR-479 制造二期生产执行与库存成本闭环验收记录

## 需求
PR-479-MANUFACTURING-PHASE2-EXECUTION-COST-CLOSED-LOOP

二期目标是把一期 `生产计划 -> 工单 -> 工序卡 -> 开始生产` 扩展为可追溯的执行闭环：Stock Entry 单据、工序卡执行、工单完工入库、生产日志、库存流水和成本拆解互相关联。

不包含三期内容：甘特图、自动产能排程、MRP 自动采购建议、行业计算器插件化。

## 实现范围
- 新增 Stock Entry 业务层和 API：`POST /api/stock-entries`、`GET /api/stock-entries`、`GET /api/stock-entries/:id`。
- 工序卡状态扩展为 `pending / ready / running / paused / completed / cancelled`，新增开始、暂停、继续、完成动作。
- 新增 `POST /api/work-orders/:id/complete`，工单完工后生成完工入库 Stock Entry、成品批次、生产日志和成本记录。
- 生产工单页展示已领料、已消耗、可退料、工序进度和成本汇总。
- 工序卡页展示开始、暂停、继续、完成、保存实际和损耗原因。
- 库存作业页新增 `Stock Entry单据`，覆盖领料到WIP、WIP退料、工单消耗、完工入库和报废/损耗。

## 测试证据
- RED：新增服务、API、schema、前端和支持层测试后，缺少二期 DTO/API/schema/helper/docs 时失败。
- GREEN：`go test ./internal/application/production -run 'TestServiceOwnsManufacturingPhase2StockEntriesAndExecutionActions|TestServiceRejectsInvalidManufacturingPhase2ExecutionCommands' -count=1 -v`
- GREEN：`go test ./internal/interfaces/http/production -run TestManufacturingPhase2StockEntryAndExecutionAPIs -count=1 -v`
- GREEN：`go test ./internal/infrastructure/postgres/production -run TestManufacturingPhase2SchemaCreatesStockEntriesAndExecutionColumns -count=1 -v`
- GREEN：`node --test src/lib/manufacturing-execution.test.js src/lib/work-orders.test.js`
- GREEN：`go test ./internal/interfaces/http/support -run TestDev479ManufacturingPhase2ExecutionContracts -count=1 -v`
- GREEN：`go test ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production ./internal/interfaces/http/support -count=1`
- GREEN：`node --test src/lib/manufacturing-execution.test.js src/lib/work-orders.test.js src/lib/produce-plan.test.js src/lib/produce-running.test.js`
- GREEN：`go test ./...`
- GREEN：`npm run build` in `frontend-vue-shell`（保留已有 Vite 大 chunk warning）
- GREEN：`scripts/verify_kferp.sh changed`
- GREEN：`git diff --check`
- GREEN browser/local：生产构建 + Mock API at `http://127.0.0.1:5194/vue-shell/` 验证 `生产工单`、`工序卡`、`库存作业 / Stock Entry单据` 三个入口；二期字段、执行按钮、Stock Entry 类型和单据行均可见，无请求失败。

## 验收场景
1. 创建生产计划并提交生成工单。
2. 工单开始生产后，在库存作业创建 Stock Entry `领料到WIP`。
3. 第一工序开始、暂停、继续、完成，并记录实际投入、实际产出、损耗和损耗原因。
4. 工单完工入库，生成 `finished_receipt` Stock Entry、成品/半成品批次、生产日志和成本记录。
5. 生产成本页查看工单成本拆解：实际物料消耗、工序实际成本和总成本。
6. 成品批次追溯到工单、工序卡、物料批次和 Stock Entry。

## 手册
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_STOCK.md`

## 待 Van 验收
- 现场确认工单执行页、工序卡页和库存作业页的操作顺序是否符合实际班组职责。
- 确认 Stock Entry 单据字段是否足够覆盖咖啡、包装盒和童装样例。
- 确认生产成本页的成本拆解粒度是否需要在后续三期进一步展开。
