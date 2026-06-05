# PR-419 BOM 使用关系收敛验收记录

## 范围
- 商品档案“被哪些 BOM 使用”只展示把该商品作为组件消耗的上层有效生产 BOM。
- 产出该商品的 BOM 不再混入商品档案使用关系列表；生产关系回到生产 BOM 的产出商品和 BOM 详情中查看。
- 列表不展示“产出该商品 / 作为组件”前缀，不展示版本号，同一上层 BOM 只展示一次，失效商品或失效 BOM 不展示。
- 生产 BOM 列表删除独立“编辑”按钮；点击 BOM 名称直接打开 BOM 设置抽屉。

## 验收项
- `BOM-000884 初晓拼配 / V002` 的“被哪些 BOM 使用”不能包含自己。
- `BOM-000884 初晓拼配 / V002` 的“被哪些 BOM 使用”不能包含未把它作为组件消耗的 `BOM-000518 初晓 生产 BOM`。
- 商品档案配置抽屉只出现一处“被哪些 BOM 使用”标题，列表行只展示 `BOM编号 BOM名称`。
- `/api/production-bom-product-usage/:product_id` 不返回 `relation_type=output`。
- `used_by_boms` 只来自组件行反查，并过滤失效商品、失效上层 BOM 和自引用。

## 证据
- RED：development 旧版本 `/api/production-bom-product-usage/518` 返回 `BOM-000518` 与 `BOM-000884` 的 `output` 关系；`/api/production-boms/884` 的 `used_by_boms` 同样包含自引用和错误 output 行。
- GREEN frontend：`node --test src/lib/bom.test.js src/lib/product-settings.test.js` 通过 120/120。
- GREEN Go/API：`go test ./internal/application/bom ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/support -count=1` 通过。
- GREEN full Go：`go test ./...` 通过。
- GREEN build：`npm run build` 通过，仅保留既有 chunk-size/plugin timing warning。
- GREEN changed verifier：`scripts/verify_kferp.sh changed` 通过。
- GREEN diff check：`git diff --check` 通过。
- 部署 smoke 与浏览器验收：待 development deploy 后补充。
