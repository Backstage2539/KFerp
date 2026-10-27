# 客户费用月结与已发货订单颜色验收

日期：2026-10-27  
需求：PR-739-CUSTOMER-FEES-MONTHLY-SHIPPED-COLOR

## 业务口径

- 未付款订单行保持红色。只有已发货且已付款时整行显示绿色；“已发货”状态文字本身显示绿色。
- 月结订单款项按销售单逐笔列示应收、已收、未收，仅未收金额计入本次应收。
- 订单货款使用折后金额；库存代发商品金额不重复计费。实际运费与成功提交的代发服务费按费用来源入账；订单运费估算显示待出账。
- 月结按上海时区上月、当月或自定义日期筛选，默认订单款项、运费、代发服务费，可选择其他费用类型。重复来源不能进入不同草稿；相同范围的草稿可刷新，锁定月结不可覆盖。
- 本次仅代码、文档与开发环境交付，不生成吴国宏正式月结、不登记收款、不发布微信小程序。

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
- 开发环境部署、页面点击验收及只读业务数据核对：待完成。

