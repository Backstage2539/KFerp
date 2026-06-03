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
- feature branch：
  - `codex/price-warning-bom-drawers-20260603` 推送提交 `567df568`。
- 合并 develop：
  - `develop` fast-forward 到 `567df568c22d8c5b7d7c86d7a1183885185d0fc1` 并推送。
- development 部署：
  - `./deploy_orderapp.sh development` 通过，Docker build 内部 `go test ./...` 通过。
  - 备份路径：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603214445`。
- smoke：
  - `docker compose ps`：`erp_orderapp`、`erp_postgres`、`erp_caddy`、`erp_docconvert` 运行中，Postgres healthy。
  - 未认证 GET `/app/` 返回 `303` 到 `/app/orders`。
  - BasicAuth GET `/app/vue-shell` 返回 `200`。
  - 需求 API 包含 `PR-404-PRICE-WARNING-BOM-DRAWERS`。

## 手册
- `OP_MANUAL_COSTING.md`：补充感叹号 tooltip、计价方式路径、固定单价/成本加成与无商品形态豁免口径。
- `OP_MANUAL_INVENTORY_MATERIALS.md`：补充生产 BOM 新布局、版本抽屉和全局规格袋材映射抽屉。
