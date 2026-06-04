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

## 部署证据
- Feature branch：`codex/materials-classification-industry-settings-20260604`
- Feature commit：`0649aa30507ae4fed33b56b282e116bb86b53c4f`
- Develop：feature commit and deployment-evidence commit have been pushed to `origin/develop`.
- Deploy command：`./deploy_orderapp.sh development`
- Backup：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604232739`
- Deploy build：Docker build 内部 `go test ./...` 通过。
- Smoke：`erp_orderapp`、`erp_caddy`、`erp_postgres`、`erp_docconvert` 运行中；`GET /app/` 返回 303 到 `/app/orders`；`GET /app/vue-shell` 返回 200；未登录访问 `/app/api/materials` 和 `/app/api/req/product` 返回 401；部署源码/文档包含 PR-414、`行业设置`、`当前仓库暂无可配置项` 和 `target_qty`；development 数据库存在 `material_classification_groups`、`material_classification_assignments`、`material_industry_field_values` 和 `materials.industry_field_template_id`。
