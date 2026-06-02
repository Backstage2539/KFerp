# PR-395 商品配置、分类模板与商品价格表

## 范围
- 商品配置模板更名为“商品配置和分类模板”，只保留商品配置模板和商品分类模板。
- 阶梯价模板、单位模板拆为商品与配方下的独立功能。
- 产品价格表更名为商品价格表。
- 商品档案和客户商品名采用单归类模型。
- 分类模板/分类项可引用阶梯价模板和单位模板，商品配置模板不一致时提示。

## 验证证据
- RED：前端目标测试先失败于缺少菜单拆分、商品价格表命名、分类模板引用字段和单归类 helper；catalog API 测试先失败于分类模板/分类项命令缺少阶梯价模板和单位模板字段。
- GREEN：
  - `node --test src/lib/product-settings.test.js src/lib/costing-bean-list-version-ui.test.js src/lib/menu-ia.test.js`
  - `go test ./internal/interfaces/http/catalog ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/costing ./internal/application/costing ./internal/infrastructure/postgres/costing -count=1`
  - `npm run build`（`orderapp-remote/frontend-vue-shell`，仅有既有 chunk-size warning）
  - `scripts/verify_kferp.sh changed`

## 手册
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/OPERATION_MANUALS.md`
