# PR-410 客户商品名重命名与商品编号展示验收记录

## 范围
- 客户商品名配置中，旧“品牌名”改为“重命名”；底层 `brand_name` 字段继续作为兼容存储字段。
- 客户商品名列表删除独立品牌名列，客户商品名列优先展示重命名。
- 客户范围商品价格表、PDF/预览和发布内容优先使用重命名作为客户商品展示名称。
- 商品档案列表展示稳定商品编号，优先 `product_code`，否则使用 `SKU-000xxx` 兼容编号。

## 验收点
- [x] 客户商品名抽屉显示 `重命名`，不再显示 `品牌名`。
- [x] 客户商品名列表没有 `品牌名` 列；填写重命名后主名称显示重命名。
- [x] 客户商品价格表候选和 PDF 组装逻辑使用 `重命名 > 客户商品名 > 商品档案名`。
- [x] 商品档案列表显示商品编号，不显示分类内序号或列表序号。

## 证据
- 前端单测：`node --test src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js`
- API/仓储测试：`go test ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support ./internal/application/catalog -count=1`
- 构建：`npm run build`
- 综合检查：`scripts/verify_kferp.sh changed`
- 部署：`./deploy_orderapp.sh development`
- Smoke：容器运行；未认证 `/app/` 返回 303；认证 `/app/vue-shell` 返回 200；需求 API 暴露 `PR-410-CUSTOMER-ALIAS-RENAME-PRICE-DISPLAY`；`/app/api/product-settings` 和 `/app/api/customer-product-aliases?active=all&q=` 返回 200。
- 操作手册：
  - `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
  - `orderapp-remote/docs/OP_MANUAL_COSTING.md`

## 说明
- 按当前约定，本轮不做浏览器/人工验收；完成代码、文档、单测/API 测试、合并和 development 部署。
