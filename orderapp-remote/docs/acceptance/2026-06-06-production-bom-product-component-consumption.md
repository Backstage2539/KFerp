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

- Pending: deploy to development and replay GoalE2E drip-bag production.
- Expected: `/api/produce/start` accepts selected `534-10`.
- Expected: `/api/produce/running/finish` writes a production log for product `534` and consumes product `532` as a finished-product component.
