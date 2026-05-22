# 销售单收款码软停用验收

日期：2026-05-22

## 范围
- PR-328-SALES-ORDER-PAYMENT-CODE-SOFT-DISABLE
- 销售单设置中的收款码“停用”只停止用于新销售单，不删除收款码记录、素材记录或已上传图片；设置页继续显示停用项并支持“启用”恢复。

## 验收点
- [x] 前端按钮语义为停用/启用，调用后刷新设置列表并提示“收款码已停用”或“收款码已启用”。
- [x] API 兼容原 `DELETE /api/settings/sales-order/payment-codes/:id` 入口，但服务和仓库层动作改为 `DeactivateSalesOrderPaymentCode`。
- [x] API 新增 `POST /api/settings/sales-order/payment-codes/:id/activate` 启用入口，服务和仓库层动作使用 `ActivateSalesOrderPaymentCode`。
- [x] 仓库层停用只执行 `active=false`，启用只执行 `active=true`，`sales_order_payment_codes` 和 `sales_order_assets` 记录仍保留。
- [x] 销售单设置读取全部收款码并展示 active 状态；销售单预览、PDF/图片生成继续只使用 active 收款码。
- [x] 审计动作记录为 `deactivate` / `activate`，操作日志展示“停用收款二维码”/“启用收款二维码”，不再把本动作显示为删除。

## 证据
- 单元/支持测试：`go test ./internal/interfaces/http/support -run TestDev328SalesOrderPaymentCodeSoftDisableWiring -count=1`
- API 测试：`go test ./internal/interfaces/http/sales -run TestSalesOrderPaymentCodeDeactivateKeepsVisibleForSettingsAndCanReactivate -count=1`
- 仓库测试：`go test ./internal/infrastructure/postgres/sales -run TestDeactivateSalesOrderPaymentCodeKeepsVisibleForSettingsAndCanReactivate -count=1`
