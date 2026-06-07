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

- [ ] 商品价格管理不出现阶梯模板维护区。
- [ ] 商品价格表可打开 `管理阶梯模板` 抽屉，新增、编辑、删除阶梯模板，并为每个档位选择价格计算模板。
- [ ] 商品价格表生成抽屉可在价格表、父类、子类、商品行设置三种计价模式。
- [ ] 平铺价格行显示计价模式来源、阶梯模板来源、Pricing Rule 版本或固定价。
- [ ] 发布快照固化 `pricing_mode`、`pricing_mode_source`、`template_tier_id`、`tier_pricing_rule_id`、`tier_pricing_rule_version`、`fixed_unit_price`。
- [ ] 录单继续只使用已发布商品价格表快照取价取单位。

## 待补充

- GREEN 测试输出。
- 本地或部署浏览器验收截图/说明。
- 部署后 `scripts/scenario_acceptance.py --allow-writes` 场景结果。
