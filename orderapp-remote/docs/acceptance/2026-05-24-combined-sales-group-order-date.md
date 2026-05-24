# PR-358 组合销售单分组订单日期验收

## 范围
- 需求：`PR-358-COMBINED-SALES-GROUP-ORDER-DATE`
- 组合销售单 PDF、预览和图片版本的订单分组首行表头。

## 验收点
- 组合销售单内容区每个订单分组第一行显示 `订单日期 2026-xx-xx`。
- 分组第一行不得显示订单号，订单号只在顶部 `关联订单` 中完整展示。
- PDF 版本和图片版本复用同一分组标题规则，展示一致。

## 证据
- `go test ./internal/infrastructure/pdf -run TestCombinedSalesOrderGroupHeaderShowsOrderDateInsteadOfOrderNo -count=1`
- `go test ./internal/interfaces/http/support -run TestDev358 -count=1`
- 手册：`OP_MANUAL_ORDER_SALES.md`
