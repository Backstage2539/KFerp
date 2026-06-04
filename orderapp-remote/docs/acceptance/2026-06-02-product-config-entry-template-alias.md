# PR-392 商品档案配置入口与模板归属改造

## Scope
- 商品档案列表收敛为“商品名即配置入口”，删除重复的生产配置/BOM 操作按钮。
- 商品档案配置抽屉维护基础信息、商品配置模板、生产 BOM、工艺路线、预期损耗率、行业字段模板和值。
- 商品配置模板由商品档案引用，产品分类只保留归类能力；旧分类模板作为 legacy fallback。
- 客户商品支持批量从商品档案创建。
- 生产 BOM 明细入口改为 Vue/Vite 内部导航，不刷新左侧菜单。

## RED Evidence
- `node --test src/lib/product-settings.test.js src/lib/view-routing.test.js` 初始失败：缺少批量客户商品 payload、商品名配置入口、商品档案模板引用、行业字段表单和 SPA BOM 跳转断言。
- `go test ./internal/interfaces/http/catalog -run 'TestCustomerProductAlias|TestProductSettingsAPIUpdatesProductTemplateAndProductionConfigIndustryFields|TestProductSettingsAPIExposesSavesAndDerivesProductConfigTemplates' -count=1` 初始失败：缺少批量客户商品 API、`product_config_template_id` 和 `industry_field_template_id` 契约。

## Implementation Notes
- `products.product_config_template_id` 新增并从旧产品子类型模板回填；商品创建和基础信息保存可写入该字段。
- `product_production_configs.industry_field_template_id` 新增；生产配置字段保存 `template_field_key`、`required`、`options_json` 快照。
- `POST /api/customer-product-aliases/batch` 支持同客户批量绑定商品档案，重复项跳过，创建和跳过均进入操作日志。
- 成本、订单生产配置快照和工单生产配置快照已带上行业字段模板和值快照；成本查询优先商品档案模板，旧分类模板 fallback。

## Verification
- PASS Targeted frontend: `node --test src/lib/product-settings.test.js src/lib/view-routing.test.js` (100/100)
- PASS Targeted backend/API: `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/infrastructure/postgres/sales ./internal/infrastructure/postgres/production ./internal/interfaces/http/support -count=1`
- PASS Frontend build: `npm run build` in `orderapp-remote/frontend-vue-shell` (existing chunk-size warning only)
- PASS Broader changed verifier: `scripts/verify_kferp.sh changed`
- SKIPPED Browser/manual acceptance: Van requested code/docs/unit/API/build verification only for this round.

## Manual Paths
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
