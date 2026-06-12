# PR-485 BOM 版本驱动的制造路线

## Scope
- 生产 BOM 的产出商品成为商品可生产关系的唯一来源。
- 商品档案只设置默认生产 BOM 主表，不保存默认 BOM 版本。
- BOM 版本选择工艺路线；生产计划解析最新可用 BOM 版本和版本路线，并冻结快照。
- 工艺路线、工序、工位/设备拆成独立页面；生产排程不提交 BOM/路线覆盖。

## Acceptance
- 商品档案 `可生产该商品的 BOM` 只由 `production_boms.output_product_id` 计算，设置默认时只提交 `default_production_bom_id`。
- 同一 BOM 只保留一个 published 最新可用版本；发布新版会归档旧 published 版本。
- 创建生产计划按 `商品 -> 默认 BOM/唯一 active 产出 BOM -> 最新 published BOM 版本 -> BOM 版本路线` 解析。多个 active 产出 BOM 且无默认、默认 BOM 失效、最新版本无路线都 fail-closed。
- 工艺路线页不出现 SKU、BOM 或 BOM 版本字段；菜单拆出 `工艺路线`、`工序`、`工位/设备`。

## Evidence
- RED：`go test ./internal/infrastructure/postgres/bom -run TestProductionBomVersionsOwnRouteAndSinglePublishedVersion -count=1`
- RED：`go test ./internal/infrastructure/postgres/production -run TestProductionPlanItemsResolveLatestUsableBomVersionRouteWithoutFallback -count=1`
- RED：`go test ./internal/interfaces/http/bom -run TestProductionBomAPIsExposeGroupsCopyVersionsAndBinding -count=1`
- RED：`node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/production-schedule.test.js`
- GREEN：`go test ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/production ./internal/interfaces/http/bom ./internal/interfaces/http/manufacturing -count=1`
- GREEN：`node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/production-schedule.test.js src/lib/process-routes.test.js src/lib/menu-ia.test.js`
- GREEN：`npm run build`
