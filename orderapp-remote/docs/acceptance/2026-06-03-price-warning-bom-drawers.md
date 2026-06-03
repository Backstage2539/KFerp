# PR-404 商品价格表 warning 与生产 BOM 抽屉交互

## 范围
- 商品价格表缺少计价方式时，API warning 统一为短文案 `未设置计价方式`。
- 商品价格表卡片不直接显示长 warning 文案，改为感叹号图标；hover/focus 后显示配置路径 tooltip。
- `green_bean`、`drip_bag` 不再作为商品价格表 warning 判定豁免条件；系统其他生豆/挂耳业务逻辑保留。
- 生产 BOM 页面重排新建、过滤、移动分组和分组 Tab；BOM 版本、规格袋材映射改为列表行按钮打开抽屉。

## RED 证据
- `go test ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing -run 'Test(ProductWithoutPricingMethodDoesNotPublishCommercialTiers|ProductWithGradientTemplateDoesNotWarnMissingPricingMethod|PricingMethodWarningDoesNotExemptProductKind|ConfiguredFixedPriceAndCostPlusDoNotWarnMissingPricingMethod|BeanListRequiresExplicitGradientTemplateForCommercialTiers|CostingCalculateAPIRequiresGradientTemplateForCommercialTiers)' -count=1`
  - 失败原因：旧实现没有 `MissingPricingMethodWarning`，且生豆/挂耳分支仍豁免缺计价方式提示。
- `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js`
  - 失败原因：商品价格表仍用 `warning-chip` 直接显示 warning 文案；生产 BOM 页面仍把过滤放在全局工具区，版本和规格袋材映射仍是底部 panel。

## GREEN 证据
- `go test ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing -run 'Test(ProductWithoutPricingMethodDoesNotPublishCommercialTiers|ProductWithGradientTemplateDoesNotWarnMissingPricingMethod|PricingMethodWarningDoesNotExemptProductKind|ConfiguredFixedPriceAndCostPlusDoNotWarnMissingPricingMethod|BeanListRequiresExplicitGradientTemplateForCommercialTiers|CostingCalculateAPIRequiresGradientTemplateForCommercialTiers)' -count=1`
  - 通过。
- `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js`
  - 通过。

## 最终验证
- `go test ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`
  - 通过。
- `npm run build`（`orderapp-remote/frontend-vue-shell`）
  - 通过；保留既有 chunk-size warning。
- `scripts/verify_kferp.sh changed`
  - 通过；命令退出码 0。

## 手册
- `OP_MANUAL_COSTING.md`：补充感叹号 tooltip、计价方式路径、固定单价/成本加成与无商品形态豁免口径。
- `OP_MANUAL_INVENTORY_MATERIALS.md`：补充生产 BOM 新布局、版本抽屉和全局规格袋材映射抽屉。
