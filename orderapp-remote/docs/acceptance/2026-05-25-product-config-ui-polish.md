# PR-373 商品配置界面样式验收记录

## 范围
- SKU设置 → 商品配置 → 商品配置模板。
- 商品配置模板列表和价格表生成规则表单布局。

## 验收点
- 商品配置模板列表使用 `product-config-row` 独立列表行，选中行有清晰选中态。
- 每个列表行展示配置名称、状态标签、单位模板名称，以及库存/报价/录单摘要标签。
- 价格表生成规则使用 `price-rule-grid` 三列布局，计价方式、价格表展示单位、取整规则三个下拉框对齐。
- “价格表展示单位”的说明使用 `field-help-tooltip` 感叹号弹出提示，不再用内联说明文字挤压下拉框。

## 证据
- 单元：`node --test src/lib/product-settings.test.js`
- API/支持：`go test ./internal/interfaces/http/support -run TestDev373 -count=1`
- 前端构建：`npm run build`（在 `orderapp-remote/frontend-vue-shell`）
- 手册：`orderapp-remote/docs/OP_MANUAL_COSTING.md`
