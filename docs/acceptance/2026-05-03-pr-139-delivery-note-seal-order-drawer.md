# PR-139 出库单公章设置与订单抽屉验收记录

日期：2026-05-03

## 需求对应

- PR-139：出库单公章设置和订单抽屉优化。
- DEV-139-01：出库单预览和 PDF 复用销售单公章资产、坐标和透明背景处理，并提供出库单页公章设置入口。
- DEV-139-02：订单列表收窄为关键列；点击订单打开抽屉展示快递信息和订单状态，并支持单订单回填单号。

## 验收证据

- 单元/API：`go test ./internal/application/sales ./internal/interfaces/http/sales ./internal/infrastructure/pdf ./internal/interfaces/http/support -count=1`
- 全量 Go 回归：`go test ./... -count=1`
- 前端单测：`node --test src/lib/*.test.js src/api/*.test.js`
- 前端构建：`npm run build`
- 差异检查：`git diff --check`

## 结果

- 出库单预览返回 `seal`，PDF 渲染可嵌入公章图片。
- `/api/orders/:id/shipping-tracking` 可按订单 ID 回填单号，并把订单发货状态更新为已发货。
- 订单列表返回最近发货批次的寄件人信息，用于“快递信息”合并展示。
- 订单列表不再展示顶部批次单号输入框，单号回填入口在订单抽屉。
- 需求、验收清单、订单列表用户手册、出库单用户手册和五张需求表已同步更新。
