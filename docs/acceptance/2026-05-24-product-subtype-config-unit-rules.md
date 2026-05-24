# PR-355-PRODUCT-SUBTYPE-CONFIG-UNIT-RULES 验收记录

## 范围
- 产品子类型保存默认阶梯价模板、工序模板、产品价格表生成规则和轻量单位换算规则。
- 单位换算规则保存库存单位、报价单位、录单单位、`unit_conversion_json` 和 `integer_unit`。
- SKU 主档保存阶梯价模板、工序模板和单位规则覆盖字段，作为后续客户规则和产品价格表生成的输入。

## 例子
- 新增产品类型“速溶咖啡”。
- 新增产品子类型“冻干速溶”。
- 配置库存单位 `kg`、报价单位 `盒`、录单单位 `盒`、`unit_conversion_json={"盒":{"kg":0.2}}`、`integer_unit=true`。
- 生成产品价格表时读取该产品子类型配置，不另建价格表模块。

## 验收证据
- Go 单元：`go test ./internal/domain/catalog -run TestNormalizeProductUnitRuleDefaultsAndIntegerUnits -count=1`
- Go 仓储源码：`go test ./internal/infrastructure/postgres/catalog -run 'TestProductSubtypeConfigAndUnitRulesPersistOnCategories|TestProductConfigOverridesPersistOnProducts' -count=1`
- API：`go test ./internal/interfaces/http/catalog -run TestProductSettingsAPIExposesAndSavesSubtypeConfigAndUnitRules -count=1`
- 前端单元：`node --test src/lib/product-settings.test.js`
- 支持/API：`go test ./internal/interfaces/http/support -run TestDev355 -count=1`
