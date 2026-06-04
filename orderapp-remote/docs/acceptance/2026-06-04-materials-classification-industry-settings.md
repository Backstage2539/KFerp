# PR-414 商品价格表、仓库设置、物料档案与行业字段

## 范围
- 商品价格表候选不再因为旧豆单 metadata code 缺失而隐藏商品。
- 普通仓库可打开仓库设置并展示空状态。
- 物料档案支持分类 Tab、新建、编辑、失效、全局单位、单数量库存补录和行业字段模板。
- 行业字段模板入口移动到 `设置 → 行业设置`。

## 验证命令
- `node --test src/lib/materials-ui.test.js src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js`
- `go test ./internal/infrastructure/postgres/materials ./internal/interfaces/http/materials ./internal/interfaces/http/stock ./internal/application/stock ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`
- `npm run build`
- `scripts/verify_kferp.sh changed`

## 手册证据
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

## 结果
- `node --test src/lib/materials-ui.test.js src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js`：通过，33/33。
- `go test ./internal/infrastructure/postgres/materials ./internal/interfaces/http/materials ./internal/interfaces/http/stock ./internal/application/stock ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`：通过。
- `go test ./...`：通过。
- `npm run build`：通过；保留既有 chunk size warning。
- `scripts/verify_kferp.sh changed`：通过。
