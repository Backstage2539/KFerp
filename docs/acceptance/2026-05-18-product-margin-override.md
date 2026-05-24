# 产品级利润率覆盖验收记录

## 需求
- PR-292-PRODUCT-MARGIN-OVERRIDE：客户SKU列表 · 公共SKU增加“产品级利润率覆盖”列。
- 单个公共SKU填写利润率后，成本试算和商用豆单预览使用该值覆盖产品子类型绑定的梯度模板利润率。
- 清空产品级利润率覆盖后，产品恢复继承产品子类型梯度模板利润率。

## 验收口径
- 产品设置 API 返回 `margin_rate_override`，并支持通过 `PUT /api/products/:id` 保存数值或用 `null` 清空。
- 成本试算从产品主档读取 `margin_rate_override`，生成梯度模板商用价格时优先使用产品覆盖利润率。
- 产品设置页“客户SKU列表 · 公共SKU”展示“利润率覆盖”输入列，留空表示继承分类模板。
- 操作手册已同步说明产品级利润率覆盖、产品子类型梯度模板继承关系和清空恢复规则。

## 证据
- 单元测试：`TestProductMarginOverrideReplacesGradientTemplateTierMargin`
- 应用层测试：`TestBeanListAppliesProductMarginOverrideBeforeCategoryTemplateMargin`
- API 测试：`TestProductSettingsAPISavesAndReturnsProductMarginOverride`
- 源码守卫：`TestProductMarginOverridePersistsOnProducts`、`TestLoadProductInputsReadsProductMarginOverrideForTemplatePricing`、`TestDev292ProductSettingsVueExposesMarginOverrideColumn`
- 验证命令：`go test ./... -count=1`
- 前端验证：`npm run build`、`node --test src/api/*.test.js src/lib/*.test.js`
- 手册路径：`OP_MANUAL_INVENTORY_MATERIALS.md`、`OP_MANUAL_COSTING.md`
