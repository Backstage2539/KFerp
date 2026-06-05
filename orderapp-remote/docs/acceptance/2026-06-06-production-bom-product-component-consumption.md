# PR-423-PRODUCTION-BOM-PRODUCT-COMPONENT-CONSUMPTION

## Scope

- 新生产 BOM `component_type=product` 组件在生产消耗层等价旧 `finished_product` 组件。
- 支持挂耳生产消耗已生产熟豆商品组件。

## Evidence

- RED live: PR-422 部署后，GoalE2E 挂耳 `534-10` 可重新进入生产计划，但 `/api/produce/start` 返回 `product BOM not configured: GoalE2E-0605-234447 咖啡挂耳`。
- Root cause: `normalizeBomComponentType` 只识别旧 `finished_product`，把新生产 BOM 的 `product` 组件归一化为 `material`，随后因 `material_id=0` 被过滤。
- RED local: `go test ./internal/infrastructure/postgres/production -run TestNormalizeBomComponentTypeAcceptsProductionBomProductComponents -count=1` failed before implementation.
- GREEN local: `go test ./internal/infrastructure/postgres/production -run 'TestNormalizeBomComponentTypeAcceptsProductionBomProductComponents|TestCurrentMaterialNeedsDeductsFinishedProductComponent' -count=1` passed.

## Deployment Acceptance

- Passed on development after deploy `6d86f5ee581966813407425a7593ea99b036c1d1`.
- `/api/produce/start` accepted selected `534-10` and created batch `A20260605-163043-0f`.
- `/api/produce/running/finish` completed product `534` with 20 units / 200g.
- Production log id `13` recorded `GoalE2E-0605-234447 咖啡挂耳` output and consumed product `532` `GoalE2E-0605-234447 咖啡熟豆` as a finished-product component.
