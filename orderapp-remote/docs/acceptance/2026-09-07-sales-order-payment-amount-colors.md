# PR-635 销售单付款金额颜色统一

日期：2026-09-07；基线：`origin/develop@179f77c6`；分支：`codex/sales-order-payment-colors-20260907`。

## 验收口径

- 全额已付、无预付款：绿色“已付金额”，值为应收合计。
- 完全未付：红色“未付金额”，值为应收合计。
- 部分预付款：保留绿色“已支付预付款”和红色“未支付尾款”；结清后保留原累计已付规则。
- 网页 ERP、小程序、普通销售单、组合销售单、PDF 和 PNG 使用同一付款快照及颜色规则。
- 历史生成文件不覆盖；新快照携带 `sales-order-payment-amount-colors-v2`，小程序不复用旧版式资产并按需生成新版本。

## TDD 证据

- RED：`TestSalesOrderPaymentRowsShowPaidAndUnpaidWithoutPrepayment` 证明普通已付/未付均没有金额行；`TestSalesOrderPaymentStatePNGColors` 证明 PNG 中缺少对应绿色/红色块。
- RED：`TestMiniEmployeeSalesOrderGenerateRefreshesStaleRenderVersion` 证明小程序会复用旧版式销售单，不会触发新版生成。
- RED：`TestDev635SalesOrderPaymentAmountColorContracts` 证明需求、手册、PR/DEV 与验收记录尚未齐全。
- GREEN：`go test ./internal/infrastructure/pdf ./internal/domain/sales -count=1` 通过；付款行、精确 PNG 填充色、预付款回归和组合销售单均通过。
- GREEN：网页 API `TestSalesOrderDocumentAPI` / `TestSalesOrderPreviewAPIDoesNotCreateDocumentVersion` 通过，返回当前渲染版本、`paid_amount/unpaid_amount`，PDF 为 `application/pdf` 且 PNG 含红色未付金额块。
- GREEN：小程序 API `TestMiniEmployeeSalesOrderGenerateRefreshesStaleRenderVersion` 通过；旧渲染版本分别触发 PDF/PNG 新版本生成，当前版本仍复用。
- GREEN：`scripts/verify_kferp.sh changed` 与 `scripts/verify_kferp.sh backend` 通过；完整 Go 包无失败。
- GREEN：`scripts/verify_kferp.sh frontend-tests` 1093/1093 通过；`scripts/verify_kferp.sh frontend-build` 6602 modules 构建成功。首次构建因隔离工作区未安装 Vite 未启动，按锁文件 `npm ci` 后通过；未执行依赖自动升级。
- GREEN：`/tmp/pr635-sales-order-payment-colors/` 生成 paid、unpaid、prepayment、combined 四组 PDF/PNG；PDF 均为 1 页，PNG 为 2480×3508。PDF 页面与原生 PNG 均已逐张检查，绿色/红色金额、文案、数字一致，无重叠、截字或底部裁切。

## 发布与验收边界

- 本轮不部署，不修改订单、付款或历史销售单业务数据。
- 产品验收人：Van；目标环境导出验收待后续安排。
