# PR-724 工位合并领料交付证据

## 需求与边界
通用工位支持任务多选、同料汇总、物料多选、实际数量修改和一键过账。生产包装台为首要验收场景；真实生产单据仅做只读验证。

## TDD
- RED：新增批量 API 测试缺少接口/类型失败，随后真实数据库请求返回 404；前端 helper 缺失失败。
- GREEN：隔离 PostgreSQL API 覆盖合并、超领、相同工单不同任务、并发同请求、改变请求内容、旧草稿、过时预览、部分库存不足、部分组失败、单组回滚和多批次、件数包材/客户货主隔离；10 个数据库场景在 race 检查下通过。
- 单测：需求优先于额外 WIP、数量精度/输入校验；权限必须同时满足生产与库存。
- 前端 helper 与真实 SFC 测试：三态选择、实际数量、结果渲染、正确单据入口、网络未知结果保留请求重试。

## Verifier
- `ORDERAPP_TEST_DATABASE_URL=... go test ./internal/interfaces/http/production -run TestBatchMaterialIssue -count=1 -v`
- `go test ./internal/application/production ./internal/interfaces/http/support -run TestBatchMaterialIssue -count=1`
- `node --test frontend-vue-shell/src/lib/workstation-batch-issue*.test.js`
- `scripts/verify_kferp.sh changed`, `backend`, `frontend-tests`, `frontend-build`

## 手册
生产手册和库存作业手册新增 PR-724 操作流程；沿用 Vue 手册入口自动读取同一源文档。操作日志增加“工位合并领料”筛选与中文标签。

## 本地检查结果
- Go 全量回归通过（数据库相关场景另用隔离 PostgreSQL 验证）。
- 前端全量 1,563 项通过；Vue 生产构建通过。
- changed / git diff --check 通过。
- 测试输出：outputs/pr724/backend.log、frontend-tests.log、database-api.log。

## 发布
待记录开发/生产 commit、备份、运行状态、页面与 API 只读 smoke。业务验收由 Van 确认，不代点真实领料。
