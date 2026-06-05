# PR-420 BOM 商品选择器 SKU 编号验收记录

## 范围
- 生产 BOM 编辑/新建抽屉的产出商品候选显示 `SKU编号 商品名`。
- BOM 版本明细中商品组件候选复用同一显示口径。
- `/api/bom/products` 返回 `product_code`，前端仍按商品 id 生成 `SKU-000xxx` 兜底。
- 商品档案“被哪些 BOM 使用”说明同步为组件反查口径，不再写“产出或消耗”。

## 验收项
- 编辑 `BOM-000884 初晓拼配 / V002` 时，产出商品候选可看到类似 `SKU-000518 初晓` 的主标签。
- 下拉中出现同名“初晓”时，用户能通过 SKU 编号区分。
- 可按 `SKU-000518` 搜索商品候选。
- 商品档案 `SKU-000518 初晓` 的“被哪些 BOM 使用”只表示把它作为组件消耗的上层 BOM。

## 证据
- RED frontend：`node --test src/lib/bom.test.js` 因 `bomProductOptionLabel` 未导出失败。
- RED API：`go test ./internal/interfaces/http/bom -run TestBomListAndProductsExposeCustomerID -count=1` 因 `Option` 缺少 `ProductCode` 字段失败。
- RED copy：`node --test src/lib/product-settings.test.js` 因抽屉说明仍含“产出或消耗”失败。
- GREEN frontend：`node --test src/lib/bom.test.js` 通过 12/12。
- GREEN product settings：`node --test src/lib/product-settings.test.js` 通过 109/109。
- GREEN API：`go test ./internal/interfaces/http/bom -run TestBomListAndProductsExposeCustomerID -count=1` 通过。
- GREEN targeted frontend：`node --test src/lib/bom.test.js src/lib/product-settings.test.js` 通过 121/121。
- GREEN targeted Go/API：`go test ./internal/application/bom ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/support -count=1` 通过。
- GREEN full Go：`go test ./...` 通过。
- GREEN build：`npm run build` 通过，仅保留既有 chunk-size/plugin timing warning。
- GREEN changed verifier：`scripts/verify_kferp.sh changed` 通过。
- GREEN diff check：`git diff --check` 通过。
- 部署 smoke 与浏览器验收：待 development deploy 后补充。
