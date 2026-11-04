# 客户费用月结与已发货订单颜色验收

日期：2026-10-27
需求：PR-739-CUSTOMER-FEES-MONTHLY-SHIPPED-COLOR
## 业务口径

- 未付款订单行保持红色。只有已发货且已付款时整行显示绿色；“已发货”状态文字本身显示绿色。
- 月结订单款项按销售单逐笔列示应收、已收、未收，仅未收金额计入本次应收。
- 订单货款使用折后金额；库存代发商品金额不重复计费。实际运费与成功提交的代发服务费按费用来源入账；订单运费估算显示待出账。
- 月结按上海时区上月、当月或自定义日期筛选，默认订单款项、运费、代发服务费，可选择其他费用类型。重复来源不能进入不同草稿；相同范围的草稿可刷新，锁定月结不可覆盖。
- 正式月结需要用户确认；本次不为吴国宏生成正式账单、不登记收款。微信小程序发布仍暂缓。

## RED / GREEN 证据

- 订单颜色 helper 测试先因实现缺失失败，补充状态计算后通过；覆盖未付款已发货、已付款已发货、已付款未发货、部分发货及作废订单。
- 月结费用类型校验及 API 传参测试先失败，加入类型验证和 `charge_types` 后通过。
- 月结区误渲染旧代发表单的 DOM 测试先失败；仅在“代发”页签显示共享表单后通过。
- PostgreSQL 定向测试复现和修复了查询/保存路径问题，之后 `CreateSettlement*` 集合通过。

## 验证结果

- 前端定向测试：`node --test src/lib/order-row-state.test.js src/lib/customer-settlement.test.js src/lib/fulfillment-operations-dom.test.js src/lib/operation-manuals.test.js`，11 项通过。
- PostgreSQL 定向测试：
  `ORDERAPP_TEST_DATABASE_URL='postgresql://yiiiple-work@127.0.0.1:5432/postgres' go test ./internal/application/customerfulfillment ./internal/infrastructure/postgres/customerfulfillment ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/customerfulfillment ./internal/infrastructure/postgres/sales -run 'Test(CreateSettlement|NormalizeSettlement|CustomerAccount|.*ProcessingBilling.*)' -count=1`，通过。
- Go 全量检查：`env -u ORDERAPP_TEST_DATABASE_URL scripts/verify_kferp.sh backend`，通过。
- PostgreSQL 定向验证：以本机 PostgreSQL 测试库运行 `TestCreateSettlementSelectedSourcesSnapshotsOnlyUnpaidAndAudits`、`TestCreateSettlementOverlappingScopesDoNotDuplicateSourcesAndRefreshDraft`、`TestOperationsWorkbenchHistoricalReadsAndUnpaginatedTodos`、`TestCustomerAccountHistorySummaryFeesAndIsolation` 和 `TestProcessingBillingPreviewConfirmSnapshotsAndIdempotency`，通过；覆盖已收/未收拆分、历史金额回退、代发商品去重、来源去重/草稿刷新、操作日志、客户账单读取、运营台汇总及结算撤销兼容。
- PostgreSQL 全包共享测试库扫描曾碰到其他历史测试夹具缺少 `product_bom_spec_authorities`、订单字段等表/列；本次相关结算、运营台汇总用例已单独在真实 PostgreSQL 测试库通过。
- Vue/Vite 生产构建：`npm run build`，通过；保留现有 PDF 预览大 chunk 警告。
- Vue/Vite 全量测试：`scripts/verify_kferp.sh frontend`，1,638 项通过，构建通过。
- `git diff --check`：通过。

## 页面与手册

- 已更新客户履约、运费、销售订单操作手册及 Vue/Vite 手册入口。
- 费用页移除库存代发、无库存代发入口；代发页签仍使用现有新建代发功能。
- 开发环境交付：功能 commit `7701bbae5d8845446a485436175439d9c0953c25` 与进度/证据 commit `2b52d05279643ead7593f72010baf9b45a30dee4` 已合入 `develop`。最终运行版本为 `2b52d05279643ead7593f72010baf9b45a30dee4`，命令 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`；发布前备份 `/opt/stacks/erp/orderapp.backup.deploy-20261027163016-2b52d0527964`，回滚镜像 `kferp-orderapp-rollback:development-20261027163016-2b52d0527964`。Vue 1,638 项、小程序 436 项、Go 全量及 Docker 镜像内 Go 全量均通过；小程序 typecheck/build 通过并校验 24 个页面。`erp_orderapp` 正常运行、PostgreSQL healthy，登录页 HTTP 200，匿名受保护 API 401，登录后 PR/DEV-PR739-04 可见且状态为 done。
- 浏览器检查：费用页的月结面板显示上月、当月、自定义日期，默认勾选订单款项、运费、代发服务费，并列出逐单应收/已收/未收。费用页没有库存代发、无库存代发旧入口；代发页签仍有“新建代发”。订单列表中未付款已发货样例行 class 为 `state-unpaid`、背景 `rgb(255, 241, 242)`；“已发货”文字为绿色 `rgb(39, 132, 86)`。已付款且已发货样例行 class 为 `state-shipped-paid`、背景 `rgb(236, 253, 243)`。页面仅筛选/查看，没有提交月结或修改业务数据。
- 吴国宏生产数据只读核对（客户 ID 488）：SO-20261022-0001 未付款 ¥71.00，SO-20261022-0002 未付款 ¥326.00；两单未收合计 ¥397.00。库存代发 SO-20261022-000116、SO-20261022-000117 均为已发货，商品货款 ¥0.00；运费记录分别 ¥9.10、¥11.10，代发服务费各 ¥3.00。现有草稿批次 1（2026-10-01 至 2026-10-22）¥6.00，批次 2（2026-10-01 至 2026-10-31）¥20.20，费用来源分别关联服务费与实际运费；未修改现有草稿。
- 生产交付：`main` 运行版本 `e89799f900cd2e334678f233b808d62d3136f253`。Vue/Vite 测试与构建、436 项小程序测试、类型检查和构建、24 个页面声明校验、服务器全量 Go 与镜像内 Go 测试均通过；生产 app running/restarts 0、PostgreSQL healthy、登录页 HTTP 200、匿名受保护 API 401。费用页面和订单行颜色随此发布生效。
- 生产启动时兼容迁移：客户 ID 488 的 2 笔旧草稿、合计 ¥26.20，已归档为 `reversed` 并释放 4 条费用来源关联；查询确认剩余旧草稿为 0，操作日志中有两条 `legacy_draft_release` 记录。没有生成正式账单或登记收款。发布前数据库备份、源码/镜像回退点及校验和记录于 `2026-10-27-customer-fees-billing.md`。
- 已登录生产工作台核对吴国宏费用页：合计未付/未出账均为 ¥423.20，已出账 ¥0.00；6 笔包括订单款项 ¥397.00、实际运费 ¥20.20、代发服务费 ¥6.00，全部显示已确认。仅查看页面，没有选择费用或出账。
- 微信小程序只完成构建验证，没有上传微信、提交审核或发布。
- 微信小程序：开发环境构建和页面校验完成；未下载包、未上传至微信、未提交审核或发布。
