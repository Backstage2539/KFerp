# PR-397 商品分类交互、客户商品、商品价格表 SQL 与行业字段

## 范围
- 商品档案/客户商品名分类工具区拆为“增加分类”和“移动分类”两张卡片。
- 客户商品名补齐搜索、启停过滤、批量停用和客户行业字段覆盖值。
- 商品价格表候选查询修复 `classification_template_id` 歧义，并优先使用客户商品行业字段覆盖值。
- 分类模板编辑区优化保存/删除位置、分类项模板并排和删除分类项回未分类。
- `生产 BOM` 移到生产管理菜单，路由 key 保持 `bom`。
- 行业字段模板改为左列表右编辑，新增字段只用文本/下拉，下拉预设用逗号。

## 验证
- Frontend RED：新增前端断言覆盖分类动作卡片、客户商品过滤/批停、行业字段模板逗号预设、行业字段列和生产 BOM 菜单归属，初始失败。
- API RED：新增接口/仓储/SQL 断言覆盖客户商品 `active/q`、批量停用、客户商品行业字段、分类项删除回未分类、商品价格表 SQL 无歧义，初始失败。
- Frontend GREEN：`node --test orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/product-bean-list-split.test.js orderapp-remote/frontend-vue-shell/src/lib/menu-ia.test.js` 通过 130/130。
- Backend GREEN：`go test ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/interfaces/http/manufacturing -count=1` 通过。
- Build GREEN：`npm run build` in `orderapp-remote/frontend-vue-shell` 通过，保留既有 chunk size warning。

## 手册
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/OPERATION_MANUALS.md`

## 备注
- 本轮按 Van 当前约定不做浏览器/人工验收。
- 旧行业字段类型不迁移、不删除；新 UI 只新增文本和下拉。
