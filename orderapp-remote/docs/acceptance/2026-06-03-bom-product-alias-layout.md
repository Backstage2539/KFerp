# PR-406 BOM、商品档案与客户商品名交互修正验收记录

## 范围
- 生产 BOM 页面收敛为一个商品 BOM列表，过滤行包含商品过滤、状态、搜索和批量失效。
- 未绑定商品的生产 BOM 合并进商品 BOM列表，显示“未绑定商品”，可勾选、复制、失效、移动分组和打开版本抽屉。
- 商品档案页面删除 `SKU归属` 行，压缩顶部说明，创建/失效/分类移动反馈改用统一 `kferp:notify` 通知。
- 商品档案和客户商品名分类操作改为 Tab 行右侧 `增加分类` 与 `移动到分类` 两个可搜索下拉。
- 客户商品名删除旧客户 SKU 收敛检查，新建客户商品改为抽屉；批量添加商品档案在该抽屉内切换模式。
- 客户商品名绑定商品档案失效时，列表标红并提示“绑定商品已失效”。

## RED 证据
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js`
  - 初始失败点包含：BOM 页面仍按旧顶部商品选择/旧批量失效卡片断言；商品档案和客户商品名仍按旧卡片式分类操作断言；客户商品名批量添加仍查找独立批量抽屉状态。

## GREEN 证据
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js src/lib/product-bean-list-split.test.js src/lib/workspace-context-pages.test.js`
  - 139/139 passed。
- `go test ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1`
  - passed，覆盖客户商品名列表返回 `product_active=false`、批量失效 API 和 PR-406 源码/需求标记。
- `npm run build` in `orderapp-remote/frontend-vue-shell`
  - passed。
- `scripts/verify_kferp.sh changed`
  - passed。

## 手册与需求文档
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

## 未执行项
- 按本轮约定，不做浏览器/人工验收。
