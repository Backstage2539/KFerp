# PR-399 产品价格表阶梯价来源修正

## Scope

- 修复产品价格表候选把旧产品分类或父分类上的阶梯价模板当作商品实际阶梯价来源的问题。
- 产品价格表实际阶梯价来源收敛为客户商品规则、客户商品规则模板、商品级覆盖和商品档案引用的商品配置模板。
- 分类模板和分类项引用的阶梯价模板只用于归类口径、默认检查和不一致提示。

## Evidence

- RED：`go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsDoesNotFallbackToCategoryGradientTemplates -count=1` 初始失败，证据为 `effective_gradient_template_id` 仍包含 `NULLIF(pc.gradient_template_id,0)`。
- GREEN：`go test ./internal/infrastructure/postgres/costing -run 'TestLoadProductInputsDoesNotFallbackToCategoryGradientTemplates|TestLoadProductInputsResolvesCustomerProductRuleTemplates' -count=1` 通过。
- GREEN：`go test ./internal/application/costing ./internal/interfaces/http/costing -count=1` 通过。

## Acceptance

- 商品未绑定商品级阶梯价覆盖、商品配置模板阶梯价或客户规则时，不会因为分类模板/分类项引用阶梯价模板而在产品价格表显示阶梯价。
- 明确绑定了客户规则、客户规则模板、商品级覆盖或商品配置模板的商品，继续按原链路生成阶梯价。
- 历史已发布价格表不回改，仍按原快照查询和下载。
