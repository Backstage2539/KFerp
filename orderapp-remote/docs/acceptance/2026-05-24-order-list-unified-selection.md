# PR-352-ORDER-LIST-UNIFIED-SELECTION 验收记录

## 范围
- 订单列表批量操作只使用一个复选框选择订单，不再按失效、发货、组合单据拆成多个功能复选框。
- 表头复选框必须是三态：未选为空，当前页正常订单全选为勾选，手动只选部分条目时显示横杆。
- 点击表头复选框一次全选当前页正常订单，再点一次取消全选。
- 失效和组合单据操作区不提供额外“清空”按钮，取消选择通过表头或行复选框完成。

## 证据
- 前端单测：`node --test src/lib/order-list-selection.test.js src/lib/combined-order-documents.test.js src/lib/view-routing.test.js`
- 支持模块：`go test ./internal/interfaces/http/support -run 'TestDev(288|350|352)' -count=1`
- 前端全量：`node --test src/lib/*.test.js src/api/*.test.js`
- 后端全量：`go test ./...`
- 前端构建：`npm run build`

## 验收清单
- [x] 订单列表每行只有一个复选框；失效、发货、组合单据共用同一批已选订单。
- [x] 表头复选框支持空、勾选、横杆三态。
- [x] 失效和组合单据区域没有“清空”按钮。
- [x] 单元测试覆盖选择 ID、三态状态、全选/取消全选逻辑。
- [x] 支持测试覆盖需求种子、订单列表接线、手册和验收文档。
