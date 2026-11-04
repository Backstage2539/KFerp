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
- 直接生成工单的袋装商品复用路径先复现商品组件缺失，现验证商品与规格均进入预留、实际扣减和原批次撤销；删除冻结组件预留会阻止开工。两种原料各 50% 且本批仅 1 克的数据库用例验证实际只扣 1 克。
- `finished_receipt_weight_test.go`：先复现磅、盎司被误识别为件数，再验证按克换算；旧直接生成工单测试改用已发布 BOM 和完整路线，分别验证新工单创建与旧工单完成。
- `production_plan_detail_test.go` 和 `bom_start_policy_test.go`：提交前提示缺少 BOM、投入或完整路线，拒绝无工序、重复顺序及不完整执行路线。

主要命令：

```sh
ORDERAPP_TEST_DATABASE_URL='postgres://…isolated…' go test ./internal/interfaces/http/production ./internal/appmain -run 'TestBOMStart|TestNewPlanFreezes|TestBOMMaterialSources|TestPR748' -count=1
node --test src/lib/*.test.js
npm run build
go test ./...
```

2026-11-04 合入 develop 历史批次转仓改动后，生产、创建器、库存及兼容回滚的定向 PostgreSQL 验证通过；前端完整测试 1705/1705、构建及不连接数据库的 Go 全量检查通过。独立 PostgreSQL 实例对当前功能版本和同一 develop 基线分别串行执行仓库级数据库回归：当前版本 185 项失败，基线 186 项失败；当前失败均在基线复现，另有一项基线失败在当前版本通过。这不是数据库全量测试全绿，失败及命令保留在交付记录中。早期并行执行多套隔离 schema 测试曾触发 PostgreSQL `out of shared memory`，该轮无效。

原需求编号 PR-747 与 develop 上已合入的历史批次转仓需求及其 DEV 编号冲突，因此本需求改为 PR-748 / DEV-PR748-01～04。测试文件名仍保留 `pr747`，方便追溯原始 RED/GREEN 证据；开发演练的数据标记使用 PR-748。

## 兼容回滚

支持本功能的镜像带有 `io.kferp.bom-start-aware=1` 标记。部署失败时，回滚前检查目标镜像和数据库：已有新策略工单时拒绝恢复没有标记的旧镜像；数据库状态不可核实时也停止应用并等待处理。恢复支持新策略的镜像时持久设置 `KFERP_BOM_START_READ_ONLY=1`，旧工单可继续，新策略工单暂停写入。经修复并重新部署后再解除暂停。上述回滚只处理代码与写入开关，不撤销已经发生的库存业务。

隔离 API 验证覆盖新策略开工受阻、库存不变与旧策略工单继续执行；`bom-start-rollback-script.test.js` 覆盖旧镜像、已存在新工单、数据库不可用和回滚标记。开发服务器当前镜像与数据库的只读检查返回 `legacy`；部署后的目标镜像标记仍需核验。

## 待完成

- 核验开发环境目标镜像的兼容标记，并保留回滚写入暂停及旧工单继续执行的验证证据。
- 完成浏览器交互、开发环境部署与使用带 PR-748 标识的隔离数据完成两套模板业务演练；核对运行提交、操作日志与 PR/DEV 可见性。
- 更新最终 commit、测试结果、环境及验收证据，产品需求保留待 Van 验收。
