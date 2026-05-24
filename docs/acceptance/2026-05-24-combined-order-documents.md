# PR-350-COMBINED-ORDER-DOCUMENTS 验收记录

## 范围
- 订单列表增加“组合单据”选择列，同一客户多张订单可生成组合销售单。
- 同一客户多张已发货订单可生成组合出库单。
- 组合销售单和组合出库单都展示关联订单、单据日期、订单日期和分组明细。

## 验收点
- 组合销售单：选择同一客户至少两张订单后，可预览 PDF；预览展示组合销售单、关联订单、每张订单的单据日期、订单日期、商品明细、小计和汇总应收；确认生成后可下载正式 PDF。
- 组合出库单：选择同一客户且均已发货的至少两张订单后，可预览 PDF；预览展示组合出库单、关联订单、每张订单的单据日期、订单日期、出库日期、收货/物流信息和商品明细；确认生成后可下载正式 PDF。
- 异常：跨客户订单、少于两张订单不能生成组合单据；组合出库单包含未发货订单时接口拒绝。
- 审计：生成组合销售单写 `combined_sales_order_document`；生成组合出库单写 `combined_delivery_note_document`，在操作日志显示为组合销售单文件或组合出库单文件。

## 证据
- 单元测试：`go test ./internal/domain/sales -run TestCombined -count=1`
- API 测试：`go test ./internal/interfaces/http/sales -run TestCombined -count=1`
- 前端单测：`node --test src/lib/combined-order-documents.test.js`
- 手册：`OP_MANUAL_ORDER_SALES.md`
