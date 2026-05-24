# PR-357 销售单收款版式跨页拖动验收

- 需求：`PR-357-SALES-ORDER-PAYMENT-OVERLAY-CROSS-PAGE`
- 场景：销售单预览因说明/收款码生成第 2 页后，操作员缩小文字框或收款码框，需要把它拖回上一页。
- 预期：
  - 续页上的“文字位置和大小”“收款码位置和大小”拖拽框向上拖过页顶后，切到上一页底部并可继续向上拖动。
  - 上一页上的收款布局框向下拖过页底后，切回下一页。
  - 松手保存后，前端保留当前页码，不会因为预览仍是多页而立刻固定回续页。
  - 刷新预览后，拖拽框仍跟随实际 PDF 页显示并可继续调整。
- 证据：
  - `node --test src/lib/document-pdf-stamp.test.js src/lib/sales-order.test.js`
  - `go test ./internal/interfaces/http/support -run TestDev357 -count=1`
  - `OP_MANUAL_ORDER_SALES.md`
