# PR-516-STANDARD-COST-DEFAULT-CAPACITY acceptance evidence

## Scope

- 工艺路线工序新增 `标准成本默认产能`，用于标准制造成本和价格试算。
- 生产计划/工单仍在执行阶段选择真实工位产能和批次，不被标准成本默认产能锁定。
- 多个启用适用产能未设置默认时，价格试算警告并拦截价格表发布。

## Evidence Targets

- Manufacturing API: `process_route_operations.standard_cost_capacity_id` 持久化、读取、审计，并校验产能存在、启用、适用于当前工序。
- Costing repository: 标准工序成本优先使用路线默认产能；唯一匹配产能可兜底并标注 `唯一匹配产能`；多候选未设置默认返回 `请为工艺路线工序设置标准成本默认产能`。
- Frontend: `生产管理 -> 工艺路线` 工序行显示 `标准成本默认产能` 下拉和 `小时费率 × 标准分钟 / 60 / 标准产出` 折算说明。
- Price list: 商品价格表发布遇到标准成本默认产能缺失警告时失败，避免发布不确定成本。

## Verification

- `go test ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/manufacturing ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing -count=1`
- `node --test src/lib/process-routes.test.js src/lib/product-settings.test.js`
- `npm run build`
- `scripts/verify_kferp.sh changed`
