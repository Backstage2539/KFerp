# PR-728 生产简化验收证据

状态：开发环境已发布，运行与资源检查通过；服务器隔离验证通过，生产发布进行中；现场真实数量核实与 Van 业务验收未进行。

## 范围

实际数量 → 一次完成 → 自动耗料／交接／入库／成本与日志。冻结路线的入库位置、组件使用工序和来源仓；内部批次余额不属于可发货库存。原入库入口改为记录与核对。执行中旧工单需显式核对后转换。

## TDD 与隔离证据

使用独立本机 PostgreSQL 16（127.0.0.1:55428），每个 API 测试独立 schema，结束清理；从未写生产业务数据。

- RED：规划器／路线配置类型缺失；完成预览 404；生产记录 404；旧单核对预览 404；执行中转换预览 404。
- RED：已完成卡仍允许再次提交建议；更正库存为 98 袋但历史汇总仍 100；商品组件错误依赖当前 BOM 配置。
- GREEN：定向 API 覆盖一次完成及重复请求、早交接与并发重放、超产／少产／零产出、整数拒绝、库存不足回滚、草稿无库存影响、指定仓商品组件、原误填 14 袋现场改为 1 袋、执行中旧单核对后继续完成。
- GREEN：100 → 98 袋更正保留原事件，库存与生产日志汇总均调整 2 袋，原料消耗不重复。指定商品组件使用 22700g 后来源仓余 7 包加 300g，批次和预留一致。
- 前端 RED／GREEN：真实数量不设计划上限、数量整数、稳定工序身份、丢失响应保留原请求、草稿不要求完成原因；旧单真实数量与用料必须填写，不能从误报或计划自动推定。

定向命令：

```
ORDERAPP_TEST_DATABASE_URL=<isolated-dsn> go test ./internal/interfaces/http/production -run '^Test(OperationCompletionAPI|OperationConversionAPI|LegacyReview)' -count=1
node --test src/lib/operation-completion.test.js src/lib/production-records.test.js
```

## 最终本地检查

- 全套 Go：`scripts/verify_kferp.sh backend` 通过。
- Vue：1,587 项测试通过；生产构建通过。构建保留既有分包大小提示。
- 新增 PostgreSQL API：全部通过，包含同单并发重放、不同后序批次争用同一余额、来源仓及货主隔离、质量冻结、失败原请求重试、计时成本、既有加工预留保护、旧耗料抵扣。
- 补充 RED/GREEN：旧耗料流水成本原为 0 而应为 22.7，修复后承接且不重复扣料；已被加工预留的成品更正明确拒绝并显示冲突记录。
- 更正保留原完成时间；多张来源卡按实际投入分配材料成本，不把整笔成本放到第一张卡。
- 扩大数据库回归存在 25 条失败测试/子用例行；在原始 `7459d68d` 与本分支逐项复现一致，无新增失败。主要为旧 BOM authority/货主测试夹具与旧领料、排班契约，不将其宣称为全量数据库通过。
- 本地真实 Vue 组件加隔离样例 API 验证：完成袋数、冻结规格、仓名、草稿、完成反馈、旧单真实数量留空、390px 窄屏。DOM 测试验证响应丢失保留原请求及输入。这些是本地 UI 证据，不代表生产业务验收。

![完成表单（隔离样例）](images/pr728/complete.png)
![窄屏表单（隔离样例）](images/pr728/mobile.png)
![旧单核对（隔离样例）](images/pr728/legacy.png)

## 发布待办与边界

待完成开发部署和服务器隔离验证、干净 main 集成、生产版本/容器/静态资源/只读列表检查。现场数量核对及 Van 业务验收尚未进行。

已获授权清理服务器可重建 Docker 构建缓存 1.789 GB，根盘可用 4.7 GB。数据库、镜像、源备份、回滚版本未清理。正式构建采用本次发布目录中的 Go 测试缓存，完成测试后清理该缓存再构建镜像，降低磁盘峰值。

手册：`docs/OP_MANUAL_PRODUCTION.md`、`docs/OP_MANUAL_STOCK.md`，页面帮助共用这些源文档。PR 保持 review 待 Van 验收。

## 开发发布与 main 集成

- PR #295 合并 develop：`d3c72ce4ab17dda52cab342202554769e4ef4bf0`。干净独立发布目录执行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development` 成功，服务未发生重启循环。
- 开发回滚：`kferp-orderapp-rollback:development-20261024142412-d3c72ce4ab17`；源备份 `/opt/stacks/erp/orderapp.backup.deploy-20261024142412-d3c72ce4ab17`。
- 未认证 `/app/` 为既有 303 跳转，受保护生产记录 API 为 401；使用运行容器凭据，工位/记录页面、记录/工序余额/工位总览/需求 API 返回 200。PR-728 可见。
- 公开资源与容器及发布目录逐字节一致：`WorkstationView-DLAjM7CU.js`（SHA-256 `43c60a267fd50a470dd4691f4822f546356971e363fad40e61f6fb960ba3fcce`）；`ProductionAcceptanceView-DN54vUBJ.js`（`55b375afde767994d5d60b76353c0fbc2cf6ac7b56c29539d72d5b8f1db39a6b`）。
- 只读检查前后工单、工序卡、库存单据/流水、新完成记录、操作日志摘要一致。
- main 集成从 `dfab3d49` 建独立分支，只引入 PR #295 功能提交，保留 main 已有 BOM 修复和历史证据。集成分支全套 Go、新增隔离数据库 API、Vue 1,587 项测试与构建通过；PR #296。
- 开发构建后再次清理授权范围的可重建缓存 1.217 GB，可用磁盘 4.3 GB。旧镜像、数据库、回滚源备份均保留。
- 服务器独立 schema API 验证：全部 PASS（完成/旧单核对/转换测试）；仅写随机测试 schema，结束自动清理。生产尚未发布，业务验收待 Van。
