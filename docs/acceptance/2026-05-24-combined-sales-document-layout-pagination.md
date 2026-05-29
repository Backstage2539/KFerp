# 2026-05-24 组合销售单版式与分页验收

## 范围
- PR-355-COMBINED-SALES-DOCUMENT-LAYOUT-PAGINATION
- 组合销售单 PDF/预览/图片版本，以及普通销售单 PDF/图片在超长内容下的收款说明区域。

## 验收点
- 组合销售单顶部按普通销售单呈现，不突出“组合单”，不展示组合单号或订单数。
- 顶部客户信息必须清楚展示客户、客户公司、联系电话、单据日期、关联订单和客户地址。
- 组合销售单内容区保留订单日期分组、商品明细、小计和备注；分组标题不显示订单号，也不再逐组重复单据日期。
- 销售单 PDF 商品内容过长或拖动收款说明区域导致第一页放不下时，自动新增续页，把说明和收款码放到续页并显示页码。
- 销售单 PNG 和组合销售单 PNG 导出为一张长图，不按 A4 分页，不裁切底部说明和收款码。

## 证据
- `go test ./internal/infrastructure/pdf -run 'CombinedSalesOrderHeaderMetaRows|CombinedSalesOrderGroupHeader|PaymentContinuation|LongImage' -count=1`
- `go test ./internal/infrastructure/pdf -run 'SalesOrder|Combined' -count=1`
- `go test ./internal/interfaces/http/support -run TestDev355 -count=1`
- 手册：`OP_MANUAL_ORDER_SALES.md`
