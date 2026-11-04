# PR-748：BOM 按批开工扣料验收记录

状态：开发与集成验证中，等待开发环境演练和 Van 验收。生产发布另行执行；不提交或开工 PP-0000000018。

## 范围与环境

- 新工单冻结 `bom_start_v1`；历史工单保持 `legacy`。首道工序以冻结路线顺序确定，每个批次确认后扣减本 BOM 投入；后续工序承接前序产出。
- PostgreSQL 测试使用本机隔离数据库及独立测试 schema。商品创建器测试使用两套正式模板节点、连线和版本的隔离快照，关联物料名称和费用使用合成测试数据；未修改环境中的模板或现有业务数据。
- 仓库转移、预留、开始确认、报工、补退料和撤销均分别核对库存与记录。测试数据包括重量原料、计数包材及带规格的袋装商品。

## RED 与定向验证

- `started_input_flow_api_test.go`：两种生豆、两锅及后序色选；分次报工、每批余额与投入成本。后序原先可处理超出确认数量，先记录失败再修复。
- `started_input_warehouse_api_test.go`：原始来源仓与已领料仓不同时，先复现错误选回原仓，再验证从 WIP 扣减。
- `started_input_adjustment_api_test.go`：补料、原批次退料、无报工撤销、重试、重新开工及部分报工后退料。退料原先未反映在耗料历史，先记录失败再验证原记录与冲销可追溯。
- `started_input_failures_api_test.go`：库存不足、质检冻结、错误货主、过期版本、审计写入失败、回滚只读开关和并发重试；检查库存、工序状态、库存单和投入事件无部分写入。
- `started_input_group_api_test.go`：合并开工整组回滚、逐任务投入及合并完成；新旧策略分组区分。
- `pr747_creator_production_integration_test.go`：使用生产现行两套模板的隔离快照，分别完成咖啡豆两层和挂耳四层 BOM 的创建、计划、开工、报工和入库；覆盖复用熟豆、咖啡粉和袋装商品，以及袋装商品的原批次退回。建档时无库存，实际开工核对库存。
- `production_plan_detail_test.go` 和 `bom_start_policy_test.go`：提交前提示缺少 BOM、投入或完整路线，拒绝无工序、重复顺序及不完整执行路线。

主要命令：

```sh
ORDERAPP_TEST_DATABASE_URL='postgres://…isolated…' go test ./internal/interfaces/http/production ./internal/appmain -run 'TestBOMStart|TestNewPlanFreezes|TestBOMMaterialSources|TestPR747' -count=1
node --test src/lib/*.test.js
npm run build
go test ./...
```

2026-11-04 合入最新 develop 到功能分支后，前端测试 1666/1666、构建及不连接数据库的 Go 全量检查通过。此前定向 PostgreSQL 两套模板与复用链路通过；后续代码调整需要最终串行重跑。并行执行多套隔离 schema 测试曾触发 PostgreSQL `out of shared memory`，该轮无效，须串行验证。

## 待完成

- 对照同一 develop 基线完成 PostgreSQL 全量回归，解释已有测试基线失败并处理新增失败；完成最终定向验证。
- 验证兼容回滚模式：旧工单可继续，新策略工单不能写入或走旧路径重复扣料。
- 完成浏览器交互、开发环境部署与使用带 PR-748 标识的隔离数据完成两套模板业务演练；核对运行提交、操作日志与 PR/DEV 可见性。
- 更新最终 commit、测试结果、环境及验收证据，产品需求保留待 Van 验收。
