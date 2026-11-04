# 复制订单详情增加金额

- 复制文本增加货款、订单内运费、合计付款；经 Van 纠正，合计付款按订单已付口径，未付款为 0，预付款取已收金额，已付款取订单应收总额。另行结算的运费标注不含于合计。
- TDD：先运行应用层和 HTTP 测试，两者均因缺少金额而失败；实现后通过。
- 验证：`go test ./internal/application/sales ./internal/interfaces/http/sales ./internal/interfaces/http/customerportal -run 'TestOrderCopyDetails|TestMiniOrderCopyDetails' -count=1 -v`，使用独立本地 PostgreSQL，覆盖数据库快照、优惠、延后运费、客户隔离和小程序接口。
- 手册：`docs/OP_MANUAL_ORDER_SALES.md`、`docs/OP_MANUAL_CUSTOMER_ACCOUNT.md`、`docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`docs/OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`；网页订单与客户账户帮助同步。
- 待 Van 在目标环境使用真实订单确认；未修改业务订单或收款记录。

- 2026-11-03 付款口径更正：应用层和 HTTP 测试先因未付款仍显示应收而失败；修复后未付款、预付款、已付款和隔离 PostgreSQL API 回归通过。待生产部署和 Van 验收。
