# PR-347-SKU-BOM-JUMP-FOCUS-FILTER SKU 列表跳转 BOM 后自动过滤验收

## 范围
- SKU设置客户SKU列表的“维护 BOM”入口。
- BOM 配方维护页的 SKU归属、商品选择和商品 BOM 列表。

## 验收点
- 从 SKU设置点击某个 SKU 行的“维护 BOM”后，URL 带 `product_id` 和 `bom_filter_product_id`。
- BOM 配方维护页自动切到该 SKU 对应的公共或客户 SKU 归属。
- 商品选择框自动选中该 SKU。
- 商品 BOM 列表只展示该 SKU 对应 BOM，不再混入同归属的其他 SKU BOM。
- 点击“显示全部 BOM”后，当前商品仍可保留选中，但列表恢复为当前归属下完整 BOM 列表。

## 验证证据
- 前端单测：`node --test src/lib/product-settings.test.js src/lib/bom.test.js`
- 支持测试：`go test ./internal/interfaces/http/support -run TestDev347 -count=1`
- 手册：`OP_MANUAL_INVENTORY_MATERIALS.md`
