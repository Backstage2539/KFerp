# 客户定制 SKU 单品利润率覆盖验收

## 需求

客户自有/客户定制 SKU 和公共 SKU 一样，可在 SKU设置 的客户SKU列表中维护单个 SKU 的产品级利润率覆盖。客户视角中引用的公共 SKU 仍只读，必须先复制为客户 SKU 后再维护客户自己的覆盖利润率。

## 验收证据

- 单元测试：`node --test src/lib/product-settings.test.js` 覆盖客户 SKU 保存 `margin_rate_override` 的请求 payload。
- API 测试：`go test ./internal/interfaces/http/catalog -run TestProductSettingsAPISavesCustomerSkuMarginOverride` 覆盖客户 SKU 返回并保存 `margin_rate_override`。
- UI 接线测试：`go test ./internal/interfaces/http/support -run TestDev292ProductSettingsVueExposesMarginOverrideColumn` 覆盖利润率覆盖列不再只在公共 SKU 视图展示，并确认公共引用行仍通过 `canEditSkuRow(row)` 禁用编辑。
- 手册：`docs/OP_MANUAL_COSTING.md` 已补充客户 SKU 利润率覆盖操作和公共引用只读边界。

## 验收项

- [x] 客户自有/客户定制 SKU 行展示“利润率覆盖”输入框。
- [x] 客户自有/客户定制 SKU 行保存后通过 `PUT /api/products/:id` 写入 `margin_rate_override`。
- [x] 清空覆盖值后恢复继承二级分类绑定梯度模板利润率。
- [x] 客户上下文中的公共 SKU 引用行只读，不允许直接修改公共 SKU 利润率。
