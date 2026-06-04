# PR-406 BOM、商品档案与客户商品交互修正验收记录

## 范围
- 生产 BOM 页面收敛为独立的生产 BOM 列表，过滤行包含状态、BOM 搜索和批量失效，不再展示商品列或商品过滤。
- 生产 BOM 作为独立配方档案展示；商品引用只在右侧 BOM 详情的“引用商品”区展示。
- 点击任意 BOM 名称会显示右侧配方明细；BOM 不因未绑定商品而不能选中、复制、失效、移动分组或维护配方。
- 商品档案页面删除 `SKU归属` 行，压缩顶部说明，创建/失效/分类移动反馈改用统一 `kferp:notify` 通知。
- 商品档案和客户商品分类操作改为 Tab 行右侧 `增加分类` 与 `移动到分类` 两个可搜索下拉；客户商品可从分类项直接移动回虚拟 `未分类`。
- 客户商品删除旧客户 SKU 收敛检查，新建客户商品改为抽屉；抽屉内不展示 `进入价格表` / `默认进入价格表` 开关；批量添加商品档案在该抽屉内切换模式，搜索框位于待选商品列表顶部。
- 客户商品绑定商品档案失效时，列表标红并提示“绑定商品已失效”。

## RED 证据
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js`
  - 初始失败点包含：BOM 页面仍按旧顶部商品选择/旧批量失效卡片断言；商品档案和客户商品仍按旧卡片式分类操作断言；客户商品批量添加仍查找独立批量抽屉状态。
- 追加回归 RED：
  - `node --test src/lib/bom.test.js`：新增断言要求 BOM 名称点击走 `openBomRowPrimary(row)`，旧实现直接打开 BOM 档案抽屉，无法恢复已绑定商品的配方明细。
  - `node --test src/lib/product-settings.test.js`：新增断言要求客户商品新建抽屉不渲染 `进入价格表` 开关、批量搜索位于列表顶部，并要求移动到 `未分类` 使用显式分类 ID；旧实现仍暴露开关且 `category_id=0` 会被下拉空值吞掉。
  - `node --test src/lib/bom.test.js`：新增断言要求未绑定生产 BOM 详情可投射为右侧配方明细，商品字段为“未绑定商品”；旧实现没有该 helper，且未绑定 BOM 选择会把 `detail` 清空。

## GREEN 证据
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js src/lib/product-bean-list-split.test.js src/lib/workspace-context-pages.test.js`
  - 139/139 passed。
- `go test ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1`
  - passed，覆盖客户商品列表返回 `product_active=false`、批量失效 API 和 PR-406 源码/需求标记。
- `npm run build` in `orderapp-remote/frontend-vue-shell`
  - passed。
- `scripts/verify_kferp.sh changed`
  - passed。

## 2026-06-04 follow-up
- 修复未绑定商品 BOM 点击后右侧配方明细消失的问题。未绑定 BOM 现在从 `/api/production-boms/:id` 读取版本和配方项，并在右侧明细中显示商品为“未绑定商品”。
- RED：`node --test src/lib/bom.test.js` failed，因为 `productionBomDetailAsRecipeDetail` 尚未导出，旧选择逻辑会清空 `detail`。
- GREEN：`node --test src/lib/bom.test.js` 9/9 passed；`node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` 118/118 passed；`npm run build` passed；`scripts/verify_kferp.sh changed` passed。
- 部署：feature commit `e5cbda1d580a1b3edaf53bf8660082f7836038d6` 已快进合入并推送 `origin/develop`，development stack 通过 `./deploy_orderapp.sh development` 部署。Docker build 期间 `go test ./...` passed；初次部署备份 `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604114317`。
- Smoke：`erp_orderapp`、`erp_postgres`、`erp_caddy`、`erp_docconvert` running；未认证 GET `/app/` 返回 303 到 `/app/orders`；认证 `/app/vue-shell` 返回 200；认证 `/app/api/production-boms?status=all` 返回 200；需求 API 暴露 `PR-406-BOM-PRODUCT-ALIAS-LAYOUT`；远端源码包含 `productionBomDetailAsRecipeDetail`。

## 2026-06-04 missing product BOM follow-up
- 修复商品档案行没有生产 BOM 时只能显示“无生产 BOM / 未维护”且所有 BOM 操作禁用的问题。现在这类行会显示 `创建BOM`，点击后调用 `/api/production-boms` 创建生产 BOM 和 V001 已发布版本，再调用 `/api/products/:id/production-bom-binding` 绑定商品，随后可维护右侧配方明细。
- RED：`node --test src/lib/bom.test.js` failed，因为 `defaultProductionBomNameForProduct` 和缺 BOM 行识别 helper 尚未导出，页面也没有创建并绑定路径。
- GREEN：`node --test src/lib/bom.test.js` 10/10 passed；`node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` 119/119 passed；`npm run build` passed；`scripts/verify_kferp.sh changed` passed。
- 状态：该方案已被下方 “independent BOM list follow-up” 替代；生产 BOM 列表不再生成商品档案行，也不再展示 `创建BOM` 行操作。

## 2026-06-04 independent BOM list follow-up
- 纠正上一轮“生产 BOM 列表合并商品行”的口径。生产 BOM 是独立配方档案，列表数据只来自 `/api/production-boms?status=all`，不再读取 `/api/bom/list` 作为主列表，不再显示商品列、商品过滤、`无生产 BOM / 未维护` 商品行或 `创建BOM` 行操作。
- 商品档案配置跳转 BOM 明细时传递 `production_bom_id`，直接打开对应生产 BOM；商品引用关系在 `/api/production-boms/:id` 的 `referenced_products` 中展示。
- RED：`node --test src/lib/bom.test.js` failed，因为 `productionBomLabel` 不能识别独立 BOM 的 `code/name/latest_version_no`，且 Vue 仍包含 `商品 BOM列表`、`商品过滤` 和 `mergeProductionBomRows`；Go targeted test failed，因为 `ProductionBomDetail` 尚未返回 `referenced_products`。
- GREEN：`node --test src/lib/bom.test.js` passed；`node --test src/lib/product-settings.test.js src/lib/view-routing.test.js` passed；`go test ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/interfaces/http/support -count=1` passed。
- 部署：feature commit `4011bbfb` 已合入并推送 `origin/develop=757decf7ccfcd397d66c8726921986ae47e66cf7`，development stack 通过 `./deploy_orderapp.sh development` 部署。Docker build 期间 `go test ./...` passed；备份 `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604130904`。
- Smoke：`erp_orderapp`、`erp_postgres`、`erp_caddy`、`erp_docconvert` running；未认证 GET `/app/` 返回 303 到 `/app/orders`；GET `/app/vue-shell` 返回 200；GET `/app/api/production-boms?status=all` 返回 200；orderapp 日志显示正常监听 `:8080`；远端前端产物包含 `生产 BOM列表`，不再包含 `商品过滤`。

## 2026-06-04 BOM detail product-return follow-up
- BOM 详情的“引用商品”改为显示商品档案商品名、商品编号和版本信息的可点击入口。点击后通过 `kferp:navigate-view` 跳转到商品档案配置，并传递临时 `returnNavigation`；商品档案左上角展示 `返回BOM编辑：{BOM 名称}`，刷新页面后该返回入口消失。
- `BOM版本` 和 `全局规格袋材映射` 移入右侧 BOM 编辑详情。列表行不再提供独立 `BOM版本` / `规格袋材映射` 按钮，也不再打开对应抽屉；页面底部仍不保留独立 panel。
- Vue 开发规范新增跨页面跳转规则：涉及业务页面跳转时必须使用 `kferp:navigate-view` + `returnNavigation`，目标页提供返回来源操作入口。
- RED：`node --test src/lib/bom.test.js` failed，因为 BOM 详情尚未包含引用商品跳转、版本/袋材映射详情区；`node --test src/lib/view-routing.test.js` failed，因为 Vue 开发规范尚未记录 `returnNavigation` 跳转规则。
- GREEN：`node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` 119/119 passed；`go test ./internal/interfaces/http/bom ./internal/interfaces/http/support -count=1` passed；`npm run build` passed；`scripts/verify_kferp.sh changed` passed。
- 部署：feature commit `c04916b8` 已合入并推送 `origin/develop=189fed6972c2953e913b0c6dcdab2bb619b59d34`，development stack 通过 `./deploy_orderapp.sh development` 部署。Docker build 期间 `go test ./...` passed；备份 `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604133129`。
- Smoke：`erp_orderapp`、`erp_postgres`、`erp_caddy`、`erp_docconvert` running；未认证 GET `/app/` 返回 303 到 `/app/orders`；GET `/app/vue-shell` 返回 200；GET `/app/api/production-boms?status=all` 返回 200；远端前端 JS 包含 `返回BOM编辑`、`referenced-product-button`、`全局规格袋材映射`、`product-return-banner` 和 `复制为新版草稿`。

## 手册与需求文档
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

## 未执行项
- 按本轮约定，不做浏览器/人工验收。
