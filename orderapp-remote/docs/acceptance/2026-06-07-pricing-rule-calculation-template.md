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
- [x] 部署后 smoke 覆盖商品价格管理入口、Pricing Rule API 字段和阶梯字段拒绝写入。
- [ ] 已登录浏览器验收覆盖商品价格管理和商品价格表生成抽屉。

## GREEN
- `node --test src/lib/product-settings.test.js` passed 123/123.
- `go test ./internal/interfaces/http/catalog -run 'TestProductPricingRuleAPI(ReplacesFinalPriceRecordMasterData|SavesCalculationTemplateWithoutQuantityTiers|RejectsQuantityTierFieldsInsideCalculationTemplate)' -count=1` passed.
- `go test ./internal/interfaces/http/support -run TestDev443PricingRuleCalculationTemplateContracts -count=1` passed.
- `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support ./internal/application/costing -count=1` passed.
- `npm run build` in `frontend-vue-shell` passed with the existing chunk-size warning.
- `git diff --check`, `scripts/verify_kferp.sh changed`, and `go test ./...` passed.
- Local browser smoke rendered 商品价格管理 with formula fields and no console/page errors. Screenshot: `/tmp/pr443-price-management-local.png`.

## Deploy
- Feature branch `codex/pricing-rule-calculation-template` pushed and fast-forwarded to `origin/develop`.
- Development deploy commit: `c08b0aa88aaddea2a5dd724590998a7242ba3e74`.
- Deploy command: `./deploy_orderapp.sh` from the clean `develop` worktree.
- Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260607214343`.
- Deploy script evidence: Vue shell build passed; miniapp `vue-tsc --noEmit` and `uni build -p mp-weixin` passed; Docker build ran container-internal `go test ./...` and rebuilt/restarted `erp_orderapp`.
- Smoke evidence: `docker compose ps` showed `erp_orderapp` Up and `erp_postgres` healthy; unauthenticated `/app/` returned `303`; authenticated `/app/vue-shell/?view=productPriceManagement` returned `200`.
- Deployed API evidence: authenticated `/app/api/product-settings` returned 9 Pricing Rules; checked rows include `formula_version` and `calculation_json`.
- Deployed rejection evidence: POST `/app/api/product-pricing-rules` with `calculation_json.min_qty` returned `400 {"error":"pricing rule must not contain quantity tiers"}`.
- Deployed frontend asset evidence: production JS contains `成本项配置`、`利润方式`、`税费方式`、`最低毛利`、`公式版本`、`试算说明`.
- Browser note: Playwright without a real ERP `auth_token` is redirected to `/app/login`; no fake token was used for final browser acceptance. Van can do final visual acceptance in an already logged-in browser session.
- Known unrelated baseline: full `scripts/verify_kferp.sh frontend-tests` fails 8 historical Vue tests on both `origin/develop=50011b74` and this feature branch in BOM/workspace/customer-portal/warehouse surfaces outside PR-443. PR-443 targeted frontend test and Vue production build passed.
