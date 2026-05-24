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

## 证据
- `go test ./internal/domain/catalog -run TestResolveProductRuleConfigMergesByPriority -count=1`
- `go test ./internal/infrastructure/postgres/catalog -run TestCustomerProductRuleTemplateSchemaPersistsTemplatesAndOverrides -count=1`
- `go test ./internal/interfaces/http/support -run TestDev356 -count=1`
