# PR-443 Pricing Rule 公式配置

## Scope
- 商品价格管理中的 Pricing Rule 只维护价格计算模板：成本来源、成本项、损耗/出率、利润方式、税费方式、最低毛利、取整、公式版本和试算说明。
- Pricing Rule 不保存数量档位；数量档位继续由阶梯模板或商品价格表生成上下文提供。
- 商品价格表生成平铺价格行时冻结 Pricing Rule 公式版本和公式配置，发布快照不回写 Pricing Rule。

## RED
- `node --test src/lib/product-settings.test.js`：实现前失败于 Pricing Rule payload 缺少 `calculation_json/formula_version`，商品价格管理页面缺少成本项、利润方式、税费方式、最低毛利、公式版本和试算说明。
- `go test ./internal/interfaces/http/catalog -run 'TestProductPricingRuleAPI(ReplacesFinalPriceRecordMasterData|SavesCalculationTemplateWithoutQuantityTiers)' -count=1`：实现前失败于 API 返回缺少 `formula_version` 和 `calculation_json`。
- `go test ./internal/interfaces/http/support -run TestDev443PricingRuleCalculationTemplateContracts -count=1`：实现前失败于 schema/docs/UI 缺少 PR-443 合同标记。

## Acceptance
- [x] 商品价格管理可保存并回显公式配置。
- [x] API 拒绝把阶梯档位字段写入 Pricing Rule 的 `calculation_json`。
- [x] 商品价格表平铺价格行的 `cost_source_snapshot` 包含 `pricing_rule_formula_version` 和 `pricing_rule_calculation`。
- [ ] 部署后浏览器验收覆盖商品价格管理和商品价格表生成抽屉。

## GREEN
- `node --test src/lib/product-settings.test.js` passed 123/123.
- `go test ./internal/interfaces/http/catalog -run 'TestProductPricingRuleAPI(ReplacesFinalPriceRecordMasterData|SavesCalculationTemplateWithoutQuantityTiers|RejectsQuantityTierFieldsInsideCalculationTemplate)' -count=1` passed.
- `go test ./internal/interfaces/http/support -run TestDev443PricingRuleCalculationTemplateContracts -count=1` passed.
- `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support ./internal/application/costing -count=1` passed.
- `npm run build` in `frontend-vue-shell` passed with the existing chunk-size warning.
- `git diff --check`, `scripts/verify_kferp.sh changed`, and `go test ./...` passed.
- Local browser smoke rendered 商品价格管理 with formula fields and no console/page errors. Screenshot: `/tmp/pr443-price-management-local.png`.
