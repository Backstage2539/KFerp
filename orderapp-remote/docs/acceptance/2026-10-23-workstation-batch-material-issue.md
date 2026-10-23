# PR-724 工位合并领料交付证据

## 需求与边界
通用工位支持任务多选、同料汇总、物料多选、实际数量修改和一键过账。生产包装台为首要验收场景；真实生产单据仅做只读验证。

## TDD
- RED：新增批量 API 测试缺少接口/类型失败，随后真实数据库请求返回 404；前端 helper 缺失失败。
- GREEN：隔离 PostgreSQL API 覆盖合并、超领、相同工单不同任务、并发同请求、改变请求内容、旧草稿、过时预览、部分库存不足、部分组失败、单组回滚和多批次、件数包材/客户货主隔离；10 个数据库场景在 race 检查下通过。
- 单测：需求优先于额外 WIP、数量精度/输入校验；权限必须同时满足生产与库存。
- 前端 helper 与真实 SFC 测试：三态选择、实际数量、结果渲染、正确单据入口、网络未知结果保留请求重试。
- 跨来源仓汇总补充 RED/GREEN：同一物料仅展示一行；实际数量按任务优先级分配，再拆回各来源仓保留独立版本提交；不同货主不合并。

## Verifier
- `ORDERAPP_TEST_DATABASE_URL=... go test ./internal/interfaces/http/production -run TestBatchMaterialIssue -count=1 -v`
- `go test ./internal/application/production ./internal/interfaces/http/support -run TestBatchMaterialIssue -count=1`
- `node --test frontend-vue-shell/src/lib/workstation-batch-issue*.test.js`
- `scripts/verify_kferp.sh changed`, `backend`, `frontend-tests`, `frontend-build`

## 手册
生产手册和库存作业手册新增 PR-724 操作流程；沿用 Vue 手册入口自动读取同一源文档。操作日志增加“工位合并领料”筛选与中文标签。

## 本地检查结果
- Go 全量回归通过（数据库相关场景另用隔离 PostgreSQL 验证）。
- 前端全量 1,564 项通过；Vue 生产构建通过。
- changed / git diff --check 通过。
- 测试输出：outputs/pr724/backend.log、frontend-tests.log、database-api.log。

## 发布
开发初版 `79f903207f3075a216a7efb720b71da7f43acb92` 已发布，登录与工位页面正常，预览 HTTP 200。包装台当前 4 个任务因本批数量待核对被明确阻止。前后来源库存、WIP、领料单和预留快照一致。

在开发数据库临时独立 schema 执行 10 个 API 场景，全部通过（93.926s）；原有业务数据快照未变，测试前后临时 schema 总数同为 7，本次创建的 schema 已清理。验证实际来源仓减少、WIP 增加、预留迁移、单据/日志一致及消耗不增加。

开发浏览器验证：单项选择后全选框半选，全选 4 个任务，抽屉展示逐任务阻止原因，切换至其他工位清除选择。真实 SFC 测试验证物料选择、数量编辑、部分成功结果、文档导航和请求重试。

开发最终版本 `de55849d1aeaf9fae8d8c16e4bb7ab32da4b5621` 已发布，含跨来源仓展示修正。GitHub 开发 PR #279、#280；外部登录 HTTP 200、未登录入口 HTTP 303 跳转登录、认证工位/需求 API HTTP 200、预览 HTTP 200。运行镜像 `sha256:e6638d5fe7677e46d022e709642c0f0dd5fdb69030e5a81abecd7e29ed2b4332`；工位资源 `WorkstationView-izvpW95b.js` 的 HTTP、容器、部署源码三方 SHA256 相同（`520f44939dd2a6d283befc1b0fc529b00740ce0c9906755bf104e4b483f671c7`）。

开发回滚：`/opt/stacks/erp/orderapp.backup.deploy-20261023220114-de55849d1aea` 与 `kferp-orderapp-rollback:development-20261023220114-de55849d1aea`。

生产从 `45faf964` 干净 main 仅集成本次两个已验证功能提交，保留生产已有修正；集成分支 Go 全量、10 个数据库场景、1,564 项前端测试、构建及 changed 均通过。补查两张 SE 的提交日志与四条库存流水，来源仓 -60 kg、WIP +60 kg，重复请求不增单、不增流水、不增消耗。

## 生产交付回执
- 生产 PR：[GitHub #281](https://github.com/Backstage2539/KFerp/pull/281)，发布 commit `5b787153bb54925c8cde7368c29341f2345f8a18`。
- 正式发布命令：`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production`；远端 Vue 1,564 项、小程序 432 项、类型检查、构建、Go 全量及镜像内二次检查全部通过。
- 运行镜像：`sha256:d06374e799f84e54c7659660adbb783ece87a65cc08ebed3ab351b80a9be4142`。应用、转换服务与 PostgreSQL 正常；外部登录 HTTP 200，未登录 `/app/` HTTP 401，认证工位/需求/批量预览 HTTP 200。
- 发布前版本：`45faf9642eb7ff8148ab2b9b4832d62eb86961f5`。回滚源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20261023221335-5b787153bb54`；回滚镜像 `kferp-orderapp-rollback:production-20261023221335-5b787153bb54`，镜像 ID `sha256:d39ee684a80be76dae659d574a928b6f79edc4e3f20b25c3847373b4e747e1b4` 与发布前一致。
- 生产实际加载 `WorkstationView-izvpW95b.js`；HTTP、运行容器、源码产物 SHA256 三方一致，值为 `520f44939dd2a6d283befc1b0fc529b00740ce0c9906755bf104e4b483f671c7`。
- 包装台 16 个任务，当前可选择 15 个；预览汇总为 5 类物料，3 类包材可领，2 类半成品等待前道；4 个旧任务明确提示本批数量待核对。页面默认勾选可领物料；测试取消/恢复物料选择、将 12 改为 13 显示额外 WIP 1，随后恢复 12。页面保留默认预览，没有点生产提交按钮。
- 发布前、API 预览后、浏览器验证后的库存/单据/预留快照摘要均为 `3fd04d82d1ae23190a4872c8cbe0a5df85acfa14da416b3ccce40e8ab862b8e9`，真实业务记录未变化。
- PR 产品状态 `review`；实现 DEV 项 `done`，现场验收项 `review`。生产与库存手册已随版发布，沿用现有 Vue 帮助入口。
- 本地证据：`outputs/pr724/production-preview.jpg`、`production-before.log`、`production-preview.log`、`production-after-ui.log`、`production-deploy.log`、`development-isolated-api.log`。
- 本回执属于发布后的文档更新，不改变上述已验证运行版本。

业务验收由 Van 确认，不代点真实领料；小程序构建检查不代表微信上传或发布。
