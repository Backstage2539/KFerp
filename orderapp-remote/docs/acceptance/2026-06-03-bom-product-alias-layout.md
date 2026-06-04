# PR-406 BOM、商品档案与客户商品名交互修正验收记录

## 范围
- 生产 BOM 页面收敛为一个商品 BOM列表，过滤行包含商品过滤、状态、搜索和批量失效。
- 未绑定商品的生产 BOM 合并进商品 BOM列表，显示“未绑定商品”，可勾选、复制、失效、移动分组和打开版本抽屉。
- 点击已绑定商品的 BOM 名称会选中该商品并显示右侧配方明细；点击未绑定商品的 BOM 名称也会显示右侧配方明细，商品字段显示“未绑定商品”。
- 商品档案页面删除 `SKU归属` 行，压缩顶部说明，创建/失效/分类移动反馈改用统一 `kferp:notify` 通知。
- 商品档案和客户商品名分类操作改为 Tab 行右侧 `增加分类` 与 `移动到分类` 两个可搜索下拉；客户商品名可从分类项直接移动回虚拟 `未分类`。
- 客户商品名删除旧客户 SKU 收敛检查，新建客户商品改为抽屉；抽屉内不展示 `进入价格表` / `默认进入价格表` 开关；批量添加商品档案在该抽屉内切换模式，搜索框位于待选商品列表顶部。
- 客户商品名绑定商品档案失效时，列表标红并提示“绑定商品已失效”。

## RED 证据
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js`
  - 初始失败点包含：BOM 页面仍按旧顶部商品选择/旧批量失效卡片断言；商品档案和客户商品名仍按旧卡片式分类操作断言；客户商品名批量添加仍查找独立批量抽屉状态。
- 追加回归 RED：
  - `node --test src/lib/bom.test.js`：新增断言要求 BOM 名称点击走 `openBomRowPrimary(row)`，旧实现直接打开 BOM 档案抽屉，无法恢复已绑定商品的配方明细。
  - `node --test src/lib/product-settings.test.js`：新增断言要求客户商品新建抽屉不渲染 `进入价格表` 开关、批量搜索位于列表顶部，并要求移动到 `未分类` 使用显式分类 ID；旧实现仍暴露开关且 `category_id=0` 会被下拉空值吞掉。
  - `node --test src/lib/bom.test.js`：新增断言要求未绑定生产 BOM 详情可投射为右侧配方明细，商品字段为“未绑定商品”；旧实现没有该 helper，且未绑定 BOM 选择会把 `detail` 清空。

## GREEN 证据
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js src/lib/product-bean-list-split.test.js src/lib/workspace-context-pages.test.js`
  - 139/139 passed。
- `go test ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1`
  - passed，覆盖客户商品名列表返回 `product_active=false`、批量失效 API 和 PR-406 源码/需求标记。
- `npm run build` in `orderapp-remote/frontend-vue-shell`
  - passed。
- `scripts/verify_kferp.sh changed`
  - passed。

## 2026-06-04 follow-up
- 修复未绑定商品 BOM 点击后右侧配方明细消失的问题。未绑定 BOM 现在从 `/api/production-boms/:id` 读取版本和配方项，并在右侧明细中显示商品为“未绑定商品”。
- RED：`node --test src/lib/bom.test.js` failed，因为 `productionBomDetailAsRecipeDetail` 尚未导出，旧选择逻辑会清空 `detail`。
- GREEN：`node --test src/lib/bom.test.js` 9/9 passed；`node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` 118/118 passed；`npm run build` passed；`scripts/verify_kferp.sh changed` passed。

## 手册与需求文档
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

## 未执行项
- 按本轮约定，不做浏览器/人工验收。
