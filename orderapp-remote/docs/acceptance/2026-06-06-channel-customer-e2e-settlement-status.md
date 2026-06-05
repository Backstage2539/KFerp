# 2026-06-06 渠道客户 E2E 状态与待结算金额验收记录

## 范围
- PR-426-CHANNEL-CUSTOMER-E2E-SETTLEMENT-STATUS
- 测试渠道客户在客户工作台/小程序提交多终端订单后，客户侧可看到生产状态、发货状态和待结算金额。

## 本地测试
- RED：`go test ./internal/infrastructure/postgres/sales ./internal/application/customerportal -run 'TestOrdersSummaryExposesPendingSettlementAmount|TestGetSettlementServicePageSummaryShowsReceivableLedger' -count=1` 曾因 `OrdersSummary` 缺少金额字段、费用中心标签仍为 `未付款金额` 失败。
- RED：`node --test src/lib/customer-portal-theme.test.js` 曾因客户工作台没有 `待结算金额` 指标失败。
- RED：`go test ./internal/infrastructure/postgres/customerfulfillment -run TestCustomerFulfillmentOptionsUseCustomerProductAliases -count=1` 曾因客户商品选项按 `product_id` 去重导致同一商品档案多个客户 SKU 被合并失败。
- RED：`go test ./internal/interfaces/http/catalog -run TestCustomerProductAliasAPIsListSaveAndDisableCustomerNames -count=1` 曾因客户商品保存接口未传递 `customer_item_code` 失败。
- GREEN：`go test ./internal/infrastructure/postgres/sales ./internal/application/customerportal -run 'TestOrdersSummaryExposesPendingSettlementAmount|TestGetSettlementServicePageSummaryShowsReceivableLedger' -count=1` 通过。
- GREEN：`go test ./internal/infrastructure/postgres/customerfulfillment -run TestCustomerFulfillmentOptionsUseCustomerProductAliases -count=1` 通过。
- GREEN：`go test ./internal/interfaces/http/catalog -run TestCustomerProductAliasAPIsListSaveAndDisableCustomerNames -count=1` 通过。
- GREEN：`node --test src/lib/customer-portal-theme.test.js` 通过。

## 待补充 Live 验收
- [ ] development 部署后创建测试渠道客户、外部账号、10 个终端收件人和 10 个客户商品 SKU。
- [ ] 客户账号从客户工作台/小程序提交 10 笔订单，订单明细保留 10 个客户商品 alias 和快照。
- [ ] ERP 生产和发货后，客户侧订单列表显示每笔订单的生产状态、发货状态和物流单号。
- [ ] 客户工作台和小程序费用中心显示待结算金额。
