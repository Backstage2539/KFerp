# PR-441 商品价格表阶梯模板与三种计价模式验收记录

## 目标

- `阶梯价模板` 在新业务中改名为 `阶梯模板`，维护入口移动到商品价格表。
- 商品价格管理只维护价格计算模板 / Pricing Rule。
- 商品价格表支持三种计价模式：按阶梯模板计算、按价格计算模板计算、固定价。
- 计价配置继承顺序为 `商品 > 子类 > 父类 > 价格表`。

## RED 证据

- `node --test src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js`：实现前失败于阶梯模板档位缺少 `pricing_rule_id`、价格表未解析三种计价模式、商品价格管理仍维护阶梯模板、商品价格表缺少阶梯模板抽屉。
- `go test ./internal/application/catalog ./internal/application/costing ./internal/interfaces/http/catalog ./internal/interfaces/http/support -run 'TestPricingRuleAndPriceTierTemplateServicesUseNewPriceListModel|TestPublishBeanListAcceptsPricingRuleAndFixedPriceModes|TestPublishBeanListRequiresPR440PriceListSnapshotMetadata|TestPriceTierTemplateAPIUsesReusableQuantityTiers|TestPriceTierTemplateAPISoftDeletesTemplate|TestDev441' -count=1`：实现前失败于后端缺少档位 Pricing Rule 字段、删除 API、模式化发布校验和 PR-441 种子/文档。

## 验收项

- [x] 商品价格管理不出现阶梯模板维护区。
- [x] 商品价格表可打开 `管理阶梯模板` 抽屉，新增、编辑、删除阶梯模板，并为每个档位选择价格计算模板。
- [x] 商品价格表生成抽屉可在价格表、父类、子类、商品行设置三种计价模式。
- [x] 平铺价格行显示计价模式来源、阶梯模板来源、Pricing Rule 版本或固定价。
- [x] 发布快照固化 `pricing_mode`、`pricing_mode_source`、`template_tier_id`、`tier_pricing_rule_id`、`tier_pricing_rule_version`、`fixed_unit_price`。
- [x] 录单继续只使用已发布商品价格表快照取价取单位。

## GREEN 证据

- `node --test src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js`：147/147 passed。
- `node --test src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js src/lib/product-bean-list-split.test.js src/lib/costing-bean-list-version-ui.test.js`：177/177 passed。
- `go test ./internal/application/catalog ./internal/application/costing ./internal/interfaces/http/catalog ./internal/interfaces/http/support -run 'TestPricingRuleAndPriceTierTemplateServicesUseNewPriceListModel|TestPublishBeanListAcceptsPricingRuleAndFixedPriceModes|TestPublishBeanListRequiresPR440PriceListSnapshotMetadata|TestPriceTierTemplateAPIUsesReusableQuantityTiers|TestPriceTierTemplateAPISoftDeletesTemplate|TestDev441' -count=1`：passed。
- `python3 scripts/scenario_acceptance.py --dry-run`：passed。
- `python3 -m py_compile scripts/scenario_acceptance.py`：passed。
- `npm run build`、`go test ./...`、`scripts/verify_kferp.sh changed`、`git diff --check`：passed。

## 部署与场景

- Feature branch `codex/price-list-tier-template-modes` 已推送；`origin/develop=1e08dccc401c9a4d7c4f450e463eba2133c1134b` 已部署到 development。
- 部署备份：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260607164915`。
- 部署脚本完成 Vue build、小程序 typecheck/build、Docker build、容器内 `go test ./...`。
- Smoke：`erp_orderapp` Up，`erp_postgres` healthy，`/app/` 返回 303，认证 `/app/vue-shell/?view=costing` 返回 200，线上 docs 暴露 PR-441 标记。
- 部署后场景脚本通过：`PR440-SCENARIO-20260607-OYCLY6` 创建并清理 customer `174`、material `50`、product `544`、group `5`、Pricing Rule `5`、tier template `5`、customer reference `5`、price-list publication `61`、order `1528`。
- 清理校验：订单已作废、价格表已撤回、商品/客户/分组/Pricing Rule/阶梯模板/客户引用已停用，原料已废弃。

## 浏览器验收

- 商品价格管理：只显示 `价格计算模板 / Pricing Rule`，没有阶梯模板或最终价格记录入口。
- 商品价格表：生成价格表抽屉显示 `Price List / Item Price 生成规则`、三种计价模式和 `商品 > 子类 > 父类 > 价格表`。
- 阶梯模板抽屉：`管理阶梯模板` 可打开，显示新建、保存、删除动作，档位行显示价格计算模板。
- 录单：保留 `选择价格表` 入口。
- 销售单 `order_id=1528`：显示 `报价来源` 和 `生产来源`。
- 截图：`/tmp/pr441-price-management.png`、`/tmp/pr441-price-list.png`、`/tmp/pr441-price-list-tier-drawer.png`、`/tmp/pr441-order-entry.png`、`/tmp/pr441-sales-order.png`。
