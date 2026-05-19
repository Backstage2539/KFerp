# 验收记录：BOM 维护隐藏生豆 SKU

日期：2026-05-20

## 背景

岩师傅客户上下文下有多个自定义 SKU，其中生豆 SKU 已经在 SKU设置 中绑定熟豆 BOM。它们不应再出现在 BOM 配方维护页，否则会被误认为需要单独维护一个生豆 BOM。

## 验收点

- BOM 配方维护选择客户后，商品 BOM 列表只展示该客户需要维护自身 BOM 的熟豆、挂耳等 SKU。
- 已绑定熟豆 BOM 的生豆 SKU 不出现在 BOM 列表和商品选择器。
- 如果某客户只有生豆 SKU、没有可维护 BOM SKU，则不出现在 BOM 客户选择下拉中。
- 后端 `/api/bom/list` 和 `/api/bom/products` 通过 `product_kind` 过滤生豆，前端仍做同样防护过滤。

## 验证证据

- `go test ./internal/application/bom ./internal/infrastructure/postgres/bom -count=1`
- `node --test src/lib/bom.test.js`
- `BomView.vue` 使用 `filterBomContextProducts` 和 `bomContextCustomerIDs` 过滤 BOM 上下文。
