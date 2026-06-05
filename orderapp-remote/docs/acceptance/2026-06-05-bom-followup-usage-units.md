# PR-418 BOM follow-up：使用关系与单位口径

## 范围
- 商品档案删除“生产反查 / 可生产 BOM”分区，只保留“被哪些 BOM 使用”只读列表。
- “被哪些 BOM 使用”同时展示 BOM 产出该商品和 BOM 作为组件消耗该商品两种关系。
- BOM 编辑抽屉展示产出数量和产出单位；草稿版本允许保存产出基准，已发布版本仍需复制为新版草稿后编辑。
- BOM 明细删除全局规格袋材映射 UI；包装材料通过组件行维护。
- 配方明细的消耗单位从全局单位字典读取，旧历史单位仅兼容显示。

## 验收点
- `BOM-001369 卡布奇诺条装 / V001` 产出商品 `卡布奇诺速溶条装` 时，商品档案的“被哪些 BOM 使用”应显示该 BOM，并标识“产出该商品”。
- 商品作为半成品组件被上层 BOM 消耗时，同一列表显示上层 BOM，并标识“作为组件”。
- 打开 BOM 编辑抽屉可以看到产出数量和产出单位字段；只有当前选中版本为草稿时允许同步保存产出基准。
- BOM 明细中不再出现“全局规格袋材映射”、保存映射或删除映射入口。
- 消耗单位下拉来自 设置 → 全局设置 → 全局单位字典。

## 证据
- RED：旧前端测试仍要求“生产反查 / 可生产 BOM”，旧 Go API 未返回 `relation_type`，`GET /api/product-settings/units` 返回 405。
- GREEN 前端：`node --test src/lib/bom.test.js src/lib/product-settings.test.js` 通过 119/119。
- GREEN Go/API：`go test ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1` 通过。
- GREEN 全量：`go test ./...`、`npm run build`、`scripts/verify_kferp.sh changed` 均通过；Vue build 只有既有 chunk-size/plugin timing warning。
- 浏览器验收：本地 build + mock API 验证 BOM 明细、BOM 编辑抽屉和商品档案配置抽屉；截图保存于 `/tmp/kferp-pr418-bom-followup-qa/product-usage-drawer.png`、`/tmp/kferp-pr418-bom-followup-qa/bom-detail-units.png`、`/tmp/kferp-pr418-bom-followup-qa/bom-edit-output-basis.png`。
