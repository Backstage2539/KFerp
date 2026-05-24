# PR-356-CUSTOMER-PRODUCT-RULE-TEMPLATES

## 范围
- 客户产品规则模板按产品子类型保存阶梯价模板、工序模板、产品价格表生成规则和单位规则。
- 客户专属覆盖按客户 + 产品子类型保存，只覆盖明确设置的字段。
- 产品价格表生成前的规则解析优先级为：客户专属覆盖 > 客户产品规则模板 > 产品子类型默认 > 产品类型默认 > 系统兜底。

## 示例
- 产品类型：速溶咖啡
- 产品子类型：冻干速溶
- 产品子类型默认：产品价格表规则 `{"mode":"subtype"}`
- 客户产品规则模板：大客户速溶规则模板，冻干速溶绑定工序模板 `22`
- 客户专属覆盖：该客户冻干速溶绑定阶梯价模板 `99`

期望解析结果：阶梯价模板取客户专属覆盖 `99`，工序模板取客户产品规则模板 `22`，产品价格表规则继续取产品子类型默认，单位规则可继续继承产品类型或产品子类型。

## 操作路径
1. 进入 SKU设置，顶部“SKU归属”选择目标履约客户。
2. 在“客户产品规则”中点击“新建规则模板”，填写模板名“大客户速溶规则模板”。
3. 在模板明细中选择产品子类型“冻干速溶”，选择阶梯价模板，填写工序模板 ID，并按需填写价格表生成规则 JSON 与单位规则 JSON。
4. 保存模板后，在“当前客户规则模板”下拉绑定该模板。
5. 如果该客户某个产品子类型要使用专属阶梯价，在“客户专属覆盖”选择“冻干速溶”并覆盖阶梯价模板；未填写的工序和单位规则继续继承模板或产品子类型默认。
6. 进入产品价格表，切到该客户范围；页面读取 `/api/costing/bean-list?customer_id=客户ID`，预览和生成时按客户规则解析。
7. 保存订单进入生产计划时，工序模板按同一优先级解析，生产行应携带解析后的 `operation_template_id`。

## 证据
- `go test ./internal/application/catalog -run TestServiceDelegatesCustomerProductRuleConfiguration -count=1`
- `go test ./internal/interfaces/http/catalog -run TestProductSettingsAPIExposesAndSavesCustomerProductRules -count=1`
- `go test ./internal/infrastructure/postgres/catalog -run TestCustomerProductRuleTemplateSchemaPersistsTemplatesAndOverrides -count=1`
- `go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsResolvesCustomerProductRuleTemplates -count=1`
- `go test ./internal/infrastructure/postgres/production -run TestUnproducedNeedsResolveCustomerProductRuleTemplates -count=1`
- `go test ./internal/interfaces/http/support -run TestDev356 -count=1`
