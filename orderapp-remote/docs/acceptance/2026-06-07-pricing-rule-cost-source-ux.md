# PR-444 Pricing Rule 成本配置简化

## Scope
- 商品价格管理的 Pricing Rule 不再暴露商品成本上下文、成本取数口径、库存成本、手工成本或最近采购成本。
- 基础成本固定为 `生产 BOM 成本（物料+工序）`。生产 BOM 成本包含 BOM 物料采购成本和已选择工序成本。
- 额外费用通过 `其他成本` KV 维护，成本名为空时忽略，同名键以后写入为准，成本价格必须是非负数字。
- 价格计算模板支持编辑和失效；失效只影响后续引用，不回写已发布商品价格表快照。

## RED
- `node --test src/lib/product-settings.test.js`：实现前失败于 Pricing Rule payload 仍保存旧 `product_cost_context/cost_components`，商品价格管理 UI 仍展示旧成本来源和成本项配置，缺少其他成本 KV、编辑和失效入口。
- `go test ./internal/interfaces/http/catalog -run 'TestProductPricingRuleAPI(ReplacesFinalPriceRecordMasterData|SavesCalculationTemplateWithoutQuantityTiers|CanDeactivateExistingTemplate)' -count=1`：实现前失败于 API 默认成本来源仍为 `product_cost_context`，响应仍带 `cost_components`。

## Acceptance
- [x] 商品价格管理只显示 `基础成本` 和 `生产 BOM 成本（物料+工序）`，不显示商品成本上下文、成本取数口径、库存成本、手工成本或最近采购成本。
- [x] Pricing Rule payload/API 保存时将旧成本来源归一为 `bom_current_cost`，并从 `calculation_json` 删除 `cost_components`。
- [x] 其他成本 KV 写入 `calculation_json.other_costs`。
- [x] 模板列表可点击 `编辑模板`，启用模板可点击 `失效`。

## GREEN
- `node --test src/lib/product-settings.test.js` passed 123/123.
- `go test ./internal/interfaces/http/catalog -run 'TestProductPricingRuleAPI(ReplacesFinalPriceRecordMasterData|SavesCalculationTemplateWithoutQuantityTiers|CanDeactivateExistingTemplate|RejectsQuantityTierFieldsInsideCalculationTemplate)' -count=1` passed.
- `go test ./internal/application/catalog -run 'TestPricingRuleAndPriceTierTemplateServicesUseNewPriceListModel' -count=1` passed.
- `go test ./internal/interfaces/http/support -run 'TestDev443PricingRuleCalculationTemplateContracts|TestDev444PricingRuleCostSourceUXContracts' -count=1` passed.
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1` passed.
- `go test ./...` in `orderapp-remote` passed.
- `npm run build` in `frontend-vue-shell` passed with the existing large chunk warning.
- `scripts/verify_kferp.sh changed` and `git diff --check` passed.
- Local browser smoke with mocked read-only APIs rendered 商品价格管理 without console errors. Visible markers: `基础成本`, `生产 BOM 成本（物料+工序）`, `其他成本`, `全局币种配置`, `编辑模板`, `失效`; forbidden markers absent: `商品成本上下文`, `成本取数口径`, `成本项配置`, `库存成本`, `手工成本`, `最近采购成本`. Screenshot: `/tmp/pr444-pricing-rule-cost-source-ux.png`.
