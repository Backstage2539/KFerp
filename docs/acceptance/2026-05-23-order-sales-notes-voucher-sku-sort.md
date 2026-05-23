# PR-341 销售单备注、收款凭证、物流总价预览、SKU 名称和排序验收

## 范围
- 销售单预览、PDF 和图片下载。
- 录单/编辑订单的物流费用、收款凭证和总价预览。
- 财务经营报告来源明细。
- SKU 设置和 BOM 配方维护的商品列表。

## 验收点
- 销售单结算区有值才展示备注，并按“快递费备注 / 订单明细备注 / 销售单备注”三类各起一行；预览、PDF 和图片口径一致。
- 录单填写物流费用后，订单合计包含物流费用，并显示货款金额和物流金额小字提示。
- 录单或编辑订单上传收款凭证后，凭证区默认收起；点击凭证摘要可打开大图预览。
- 财务经营报告来源明细中，订单收入行可看到并打开收款凭证。
- SKU 设置客户 SKU 列表可直接修改 SKU 商品名称并保存。
- 客户 SKU 列表和 BOM 商品列表在客户上下文中先展示当前客户自定义商品，再按历史订单使用次数把常用商品排前。

## 验证证据
- 前端单元测试：`node --test src/lib/order-entry.test.js src/lib/product-settings.test.js src/lib/bom.test.js`
- Go/API/仓储测试：`go test ./internal/infrastructure/pdf ./internal/infrastructure/postgres/sales ./internal/infrastructure/postgres/finance ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/bom ./internal/interfaces/http/catalog ./internal/interfaces/http/finance ./internal/interfaces/http/support -run 'TestDev341|TestProductSettingsAPIUpdatesProductName|TestFinanceSourceDetailsIncludesOrderPaymentVoucher|TestSalesOrderPreviewIncludesNoteAndDiscountBreakdowns' -count=1`
- 手册：`OP_MANUAL_ORDER_SALES.md`、`OP_MANUAL_FINANCE.md`、`OP_MANUAL_INVENTORY_MATERIALS.md`
