# PR-555 生产计划草稿撤销验收记录

## 目标

生产计划生成草稿后允许用户撤销，保留原生产计划及冻结快照，并让仍满足生产条件的来源订单商品回到“待生产需求”。已提交工单、生产中和已完成计划不在本次撤销范围。

## 业务口径

- 接口：`POST /api/production-plans/{id}/cancel`
- 状态：仅允许 `draft -> cancelled`
- 数据：保留计划头、计划行、规格换算、BOM、物料、工艺路线、生产配置和工序产能拆分快照
- 回流：待生产查询忽略 `cancelled` 计划，来源需求重新派生为 `unplanned`
- 审计：同一事务记录生产计划号、计划行数和 `draft -> cancelled`
- 幂等：重复撤销返回原已取消单据，不重复写操作日志
- 边界：已有工单或任何非草稿计划拒绝；不自动撤销任何真实草稿

## RED / GREEN

- RED frontend：`node --test src/lib/produce-plan.test.js`
  - 缺少 `productionPlanCancelEndpoint`
  - 缺少当前计划、单据列表和详情抽屉的“撤销草稿”
- RED backend：`go test ./internal/interfaces/http/production -run TestProductionPlanDraftCancelRouteReturnsCancelledPlan -count=1`
  - `/api/production-plans/41/cancel` 返回 404
- GREEN targeted：
  - `node --test src/lib/produce-plan.test.js`：43/43
  - `go test ./internal/application/production ./internal/interfaces/http/production ./internal/interfaces/http/support -count=1`
- GREEN local：
  - `go test ./... -count=1`
  - `scripts/verify_kferp.sh changed`
  - `npm run build`
- 完整前端：806/813；7 个 customer/workspace 既有契约失败与 PR-555 改动文件不重叠，和当前既有基线一致。
- 真实 PostgreSQL / development smoke：待集成与部署后补充。

## 自动化验收矩阵

- 草稿撤销后状态为 `cancelled` 且 `cancelled_at` 非空。
- 计划行和冻结快照保留；工单、工序卡、running item、WIP 占用和库存单据均不新增。
- 撤销后来源订单重新显示 `unplanned / demand_selectable=true`，可创建新草稿；旧已取消计划继续可查询。
- 重复撤销只产生一条 `production_plan/cancel` 操作日志。
- 并发双撤销均幂等成功且只写一次日志；提交和撤销并发时只允许一个状态转换成功。
- submitted 计划和异常已有 work order 的 draft 计划拒绝撤销并保持原状态；工单即使只关联计划行也不能漏过校验。
- 操作日志写入失败时状态更新回滚；撤销后工序产能拆分快照继续保留。
- 撤销当前工作台草稿时清空旧预览和选择、切回“待计划”；从列表撤销其他草稿不破坏当前工作台。
- 待生产需求刷新使用请求序列保护，撤销前的旧响应不能覆盖撤销后的回流结果；失败时保留原草稿。

## 开发环境验收

- 部署目标：development
- production：不部署、不写数据
- 自动化 API / 页面只读冒烟：待部署后补充。
- 人工业务写：不自动撤销现有生产计划草稿，由用户在页面确认后执行。
