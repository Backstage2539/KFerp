# PR-405 生产 BOM 批量失效

## 范围
- 生产 BOM 页删除单独的 `失效当前 BOM` 入口。
- 商品 BOM列表支持勾选一个或多个 BOM 后点击 `批量失效`。
- 行内 `失效` 和 `批量失效` 都直接执行，不弹确认框。
- 生产 BOM 失效不再因启用商品引用被后端拒绝；商品档案读取到失效 BOM 时继续提示用户处理。

## RED
- `node --test src/lib/bom.test.js`
  - 初始失败：页面仍存在 `失效当前 BOM`，行内失效仍使用 `window.confirm`，没有批量失效卡片。
- `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/support -run 'TestProductionBomCanDeactivateWhenActiveProductsReferenceIt|TestDev167VueShowsProductMultiDeactivateAndBomInactiveWarnings' -count=1`
  - 初始失败：`UpdateProductionBom` 仍保留启用商品引用 guard，支持测试仍找不到批量失效入口。

## GREEN
- `node --test src/lib/bom.test.js` 通过。
- `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/support -count=1` 通过。
- `go test ./...` 通过。
- `npm run build` 通过，保留既有 chunk-size / plugin timing warning。
- `scripts/verify_kferp.sh changed` 通过。

## 文档
- `OP_MANUAL_INVENTORY_MATERIALS.md`
- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
