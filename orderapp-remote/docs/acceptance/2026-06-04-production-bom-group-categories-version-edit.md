# PR-407 生产 BOM 大组/组内分类与版本配方编辑验收记录

## 范围
- 生产 BOM 自定义大组内新增组内分类。
- 配方明细、合计比例和保存组件归属当前 BOM 版本。
- 新建生产 BOM 默认生成 `V001 草稿`。
- 空初始已发布版本安全修复为草稿。

## 需求映射
- PR：`PR-407-PRODUCTION-BOM-GROUP-CATEGORIES-VERSION-EDIT`
- DEV：
  - `DEV-407-BOM-GROUP-CATEGORIES-DATA-API`
  - `DEV-407-BOM-VERSION-DRAFT-RECIPE`
  - `DEV-407-BOM-VUE-GROUP-CATEGORY-UX`
  - `DEV-407-BOM-VUE-VERSION-RECIPE-UX`
  - `DEV-407-MANUAL-DOCS`

## RED 证据
- `node --test src/lib/bom.test.js`
  - 预期失败：生产 BOM 页面缺少组内分类维护入口、`groupProductionBomRowsByInnerCategory`、版本下配方编辑区和已发布只读提示。
- `go test ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom -count=1`
  - 预期失败：生产 BOM 组内分类类型、API 字段、schema 和初始草稿版本修复逻辑尚未实现。

## GREEN 证据
- `node --test src/lib/bom.test.js`
  - 通过：10/10。
- `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom -count=1`
  - 通过。
- `npm run build`
  - 通过；保留既有 chunk-size warning。
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js`
  - 通过：120/120。
- `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/interfaces/http/support -count=1`
  - 通过。
- `scripts/verify_kferp.sh changed`
  - 通过；命令退出码 0。
- `go test ./...`
  - 通过。
- 提交前复核：
  - `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` 通过：120/120。
  - `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/interfaces/http/support -count=1` 通过。
  - `npm run build` 通过；保留既有 chunk-size warning。
  - `scripts/verify_kferp.sh changed` 通过；命令退出码 0。

## 手册
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

## 待追加
- feature branch push / merge develop / development stack deploy
- deploy smoke evidence
