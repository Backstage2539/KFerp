# 销售单收款码软停用验收

日期：2026-05-22

## 范围
- PR-328-SALES-ORDER-PAYMENT-CODE-SOFT-DISABLE
- 销售单设置中的收款码“停用”只隐藏，不删除收款码记录、素材记录或已上传图片。

## 验收点
- [x] 前端按钮语义为停用，调用后刷新设置列表并提示“收款码已停用”。
- [x] API 兼容原 `DELETE /api/settings/sales-order/payment-codes/:id` 入口，但服务和仓库层动作改为 `DeactivateSalesOrderPaymentCode`。
- [x] 仓库层只执行 `active=false`，`sales_order_payment_codes` 和 `sales_order_assets` 记录仍保留。
- [x] 销售单设置、预览、PDF/图片生成继续只读取 active 收款码，停用后的收款码不再展示。
- [x] 审计动作记录为 `deactivate`，操作日志展示“停用收款二维码”，不再把本动作显示为删除。

## 证据
- 单元/支持测试：`go test ./internal/interfaces/http/support -run TestDev328SalesOrderPaymentCodeSoftDisableWiring -count=1`
- API 测试：`go test ./internal/interfaces/http/sales -run TestSalesOrderPaymentCodeDeactivateHidesWithoutDeletingRecordOrAsset -count=1`
- 仓库测试：`go test ./internal/infrastructure/postgres/sales -run TestDeactivateSalesOrderPaymentCodeHidesWithoutDeletingRecordOrAsset -count=1`
