# 2026-05-24 销售单收款拖拽框多页预览验收

## 范围
- PR-356-SALES-ORDER-PAYMENT-OVERLAY-MULTIPAGE
- 销售单预览 PDF 中“文字位置和大小”“收款码位置和大小”的拖动与拉伸框。

## 验收点
- 销售单 PDF 因说明/收款码自动生成第 2 页时，两个拖拽框显示在第 2 页，而不是按第 1 页坐标跑出可见区域。
- 历史保存的超页底坐标在预览中会被夹回当前 PDF 页可见区域，操作员仍能拖动和拉伸。
- 拖动或调整大小后保存，刷新预览后拖拽框仍可见，后续可继续修改。

## 证据
- `node --test src/lib/document-pdf-stamp.test.js src/lib/sales-order.test.js`
- `go test ./internal/interfaces/http/support -run TestDev356 -count=1`
- 手册：`OP_MANUAL_ORDER_SALES.md`
