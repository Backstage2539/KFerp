# PR-398-BOM-INDUSTRY-TEMPLATE-REFINE

## 范围
- 生产 BOM 页面新增“生产 BOM 档案”管理区，支持新建、编辑、失效、复制 BOM。
- 生产 BOM 档案支持启用/已失效/全部过滤、BOM 名称或编号搜索；复制失效 BOM 不被前端禁用。
- 行业字段模板去掉行业键、显示名、单位和必填入口；字段键即显示名。
- 行业字段新增只暴露文本/下拉；文本可填默认文本，下拉用空格分隔选项。

## RED
- `node --test orderapp-remote/frontend-vue-shell/src/lib/bom.test.js orderapp-remote/frontend-vue-shell/src/lib/product-bean-list-split.test.js`
  - 初始失败：`filterProductionBomCatalog` 未导出；行业字段模板页面仍显示行业键、显示名、单位、必填，并且下拉预设仍是逗号。
- `go test ./internal/interfaces/http/bom ./internal/interfaces/http/manufacturing -count=1`
  - 初始失败：行业字段模板保存不传 `label` 时返回 `field_key and label required`。
- `go test ./internal/infrastructure/postgres/bom -run TestProductionBomCannotDeactivateWhenActiveProductsReferenceIt -count=1`
  - 初始失败：生产 BOM 失效前未检查启用商品引用。

## GREEN
- `node --test orderapp-remote/frontend-vue-shell/src/lib/bom.test.js orderapp-remote/frontend-vue-shell/src/lib/product-bean-list-split.test.js`
  - 22/22 通过。
- `go test ./internal/interfaces/http/bom ./internal/interfaces/http/manufacturing -count=1`
  - 通过。
- `node --test src/lib/product-settings.test.js src/lib/bom.test.js src/lib/product-bean-list-split.test.js`（在 `orderapp-remote/frontend-vue-shell`）
  - 124/124 通过。
- `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/manufacturing ./internal/interfaces/http/support -count=1`
  - 通过。
- `npm run build`（在 `orderapp-remote/frontend-vue-shell`）
  - 通过；仅有既有 Vite chunk size warning。
- `scripts/verify_kferp.sh changed`
  - exit 0。
- `go test ./...`（在 `orderapp-remote`）
  - 通过。

## 待最终验证
- 合入 `develop` 后部署 development stack。

## 手册
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OPERATION_MANUALS.md`
