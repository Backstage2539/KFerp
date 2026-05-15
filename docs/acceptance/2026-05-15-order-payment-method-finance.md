# 验收记录：订单收款方式同步财务

日期：2026-05-15

## 需求
- PR-282-ORDER-PAYMENT-METHOD-FINANCE：订单改为已付款、已收款或已支付时必须选择收款方式，并同步到财务管理来源明细和会计交接数据。

## 验收项
- [ ] 订单录入和编辑：收款状态为已付款/已收款/已支付时，未选择收款方式不能保存。
- [ ] 订单 API：`POST /api/order` 编辑已付款订单时，缺少 `payment_method` 返回 `payment_method required`；补齐后保存成功。
- [ ] 订单列表和编辑回显：保存后的订单返回 `payment_method`，列表状态显示收款方式。
- [ ] 财务来源明细：订单收入行返回并展示订单收款方式。
- [ ] 会计交接 Excel：Drilldown 工作表包含 Payment method 列，订单收入行写入该收款方式。
- [ ] 操作手册：订单销售和财务管理手册已同步收款方式流程、校验和排障说明。

## 证据
- 单元测试：`node --test src/lib/order-entry.test.js`
- API 测试：`go test ./internal/interfaces/http/sales -run TestOrderAPIEditsPaidOrderRequirePaymentMethodAndExposeToList -count=1`
- 财务数据测试：`go test ./internal/infrastructure/postgres/finance -run TestFinanceSourceDetailsIncludesOrderPaymentMethod -count=1`
- 手册路径：`OP_MANUAL_ORDER_SALES.md`、`OP_MANUAL_FINANCE.md`、`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`、`orderapp-remote/docs/OP_MANUAL_FINANCE.md`
