# PR-408 商品档案新增后配置入口修正验收记录

## 范围
- 新增商品档案成功后自动打开“商品档案配置”抽屉。
- 配置抽屉使用重新加载后的完整商品行，避免刚新增商品无法配置商品配置模板、生产 BOM、工艺路线和行业字段。

## 需求映射
- PR：`PR-408-PRODUCT-CREATE-CONFIG-DRAWER`
- DEV：
  - `DEV-408-PRODUCT-CREATE-OPEN-CONFIG`

## RED 证据
- `node --test src/lib/product-settings.test.js`
  - 预期失败：`resolveCreatedProductForConfig` 尚未导出，`createSku` 未使用创建响应定位并打开商品档案配置抽屉。

## GREEN 证据
- `node --test src/lib/product-settings.test.js`
  - 通过：102/102。
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js`
  - 通过：121/121。
- `go test ./internal/interfaces/http/catalog -run TestProductSettingsAPICreatesUnifiedSKUWithoutLegacyFields -count=1`
  - 通过。
- `npm run build`
  - 通过；保留既有 chunk-size warning。
- `scripts/verify_kferp.sh changed`
  - 通过；命令退出码 0。
- `go test ./internal/interfaces/http/support -run TestDev408 -count=1`
  - 通过，覆盖 PR/DEV/UT/API/REV 需求表种子和源码 marker。
- `go test ./internal/interfaces/http/support -count=1`
  - 通过。

## 手册
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
