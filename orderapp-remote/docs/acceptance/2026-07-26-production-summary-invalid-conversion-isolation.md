# PR-554-PRODUCTION-SUMMARY-CONVERSION-ISOLATION 生产需求无效换算行隔离验收

## 问题复现

- development 打开 `生产管理 -> 生产流程 -> 生产计划` 时，未生产需求接口整体返回错误：`order CDS-20260526-1186 / product Codex测试速溶盒装 10条/盒 / spec 件: concrete SKU has no valid inventory unit conversion`。
- 只读数据检查确认：该订单对应的是停用历史测试商品；订单销售单位为“件”，商品库存单位为“盒”，没有同一具体 SKU 的权威换算。
- 原实现遇到任意单行换算错误就中断整个汇总，因此用户看不到同一范围内其他有效待生产订单。

## 修复口径

- 停用商品不进入新的未生产订单和代加工生产需求，历史订单本身不修改。
- 启用商品缺少换算时保留为可见“资料待完善”行，返回明确 `blocking_reason`，但禁止选择、预览和生成计划。
- 其他有效需求继续返回并保持可选择；单行商品资料问题不能拖垮整页。
- 系统不从“10条/盒”“件”等文本猜测数量，也不假定 `1件 = 1盒`。生产数量只能使用订单冻结快照或同一具体 SKU 的权威商品档案换算。
- 数据库、SQL 或连接错误继续按系统错误返回，不通过行级降级掩盖。

## RED / GREEN 证据

- RED：真实 PostgreSQL API 测试同时写入一条有效 454g 需求、一条启用但无换算需求和一条停用商品需求；修复前 `GET /api/produce/unproduced` 返回 500。
- GREEN：`ORDERAPP_TEST_DATABASE_URL=... go test ./internal/interfaces/http/production -run '^TestProducePlanSummaryKeepsValidDemandWhenAnotherOrderHasInvalidInventoryConversion$' -count=1` 通过，验证有效行可选、启用无换算行可见且不可选、停用行被排除，强制提交阻塞行返回 400 且计划数为 0。
- GREEN：`node --test src/lib/produce-plan.test.js` 40/40 通过，验证“资料待完善”行不参与全选或三态选择，页面保留明确原因。
- GREEN：`go test ./internal/interfaces/http/support -run '^TestDev554ProductionSummaryConversionIsolationContracts$' -count=1` 通过，验证 PR/DEV/REV、需求、手册、前端和后端关键合同一致。
- GREEN：真实 PostgreSQL 下 `go test ./internal/infrastructure/postgres/production ./internal/interfaces/http/production -count=1` 通过；`scripts/verify_kferp.sh backend`、`scripts/verify_kferp.sh changed`、Vue/Vite production build 和 `git diff --check` 通过。
- REVIEW：独立只读复核确认阻断行不可选择、库存分配跳过、强制创建零写入、前端不会清空有效行；订单快照单位优先于目录回退，API 行字段与阻断文案保持一致，无阻断发现。

## 数据与部署边界

- 本次测试使用临时 PostgreSQL 数据；不修改 `CDS-20260526-1186`，不补写商品换算，不创建真实生产计划。
- 不自动修复历史订单、商品档案、BOM、工单、库存或生产日志。
- 功能提交 `b1a65ab4` 已推送，合并到 `develop` 的提交为 `b775375a`；使用 `./deploy_orderapp.sh development` 完成 development 部署，备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260726145520`。
- Docker 镜像内完整 Go 测试通过；`erp_orderapp` 重启数为 0，`erp_postgres` 为 healthy，部署后日志中 `panic/fatal`、`SQLSTATE`、`conn busy` 和 error 标记均为 0。
- 认证 Vue shell 与需求 API 返回 200，PR-554 和部署源码标记可读。`/api/produce/unproduced` 返回 200、6 条有效可选需求、0 条阻塞需求；`CDS-20260526-1186` 不在结果中。
- 应用内浏览器访问 development 时被本地 CA 的 `ERR_CERT_AUTHORITY_INVALID` 安全页拦截；未绕过证书安全页，页面视觉交由 Van 手工确认。
- production 明确不部署、不写业务数据。
