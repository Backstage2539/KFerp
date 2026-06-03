# PR-403 BOM、价格表提示与行业字段模板修正

## 范围
- 商品价格表缺少阶梯价模板时，提示改为“未设置计价方式”，并写清从主菜单到具体下拉项的配置路径。
- 行业字段模板改为左侧模板列表、右侧编辑；模板列表支持搜索模板名和启用/停用/全部过滤。
- 生产 BOM 的新建、状态过滤、搜索、分组 Tab、移动分组卡片放到 BOM 列表与编辑详情共同顶部；BOM 列表独立滚动。
- 修复 `Codex测试豆` 这类旧 BOM 有明细但没有生产 BOM 绑定，导致不能勾选移动分组的问题。

## RED 证据
- `go test ./internal/domain/costing -run TestProductWithoutGradientTemplateDoesNotPublishCommercialTiers -count=1`
  - 失败原因：测试期望新的“未设置计价方式”提示，旧实现仍返回“未配置阶梯价模板”。
- `go test ./internal/infrastructure/postgres/bom -run TestProductionBomBackfillRepairsLegacyItemsWithoutBindings -count=1`
  - 失败原因：旧实现没有 legacy BOM 绑定修复标记和修复 SQL。
- `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js`
  - 失败原因：BOM 页面缺少共同操作区和列表滚动标记；行业字段模板缺少模板搜索、状态过滤和左列表右编辑测试标记。

## GREEN 证据
- `go test ./internal/domain/costing -run TestProductWithoutGradientTemplateDoesNotPublishCommercialTiers -count=1`
  - 通过。
- `go test ./internal/infrastructure/postgres/bom -run TestProductionBomBackfillRepairsLegacyItemsWithoutBindings -count=1`
  - 通过。
- `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js`
  - 通过。
- `npm run build`（`orderapp-remote/frontend-vue-shell`）
  - 通过；保留既有 chunk-size warning。
- `go test ./internal/domain/costing ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/manufacturing ./internal/interfaces/http/support -count=1`
  - 通过。

## 最终验证
- `go test ./...`（`orderapp-remote`）
  - 通过。
- `scripts/verify_kferp.sh changed`
  - 通过。
- 合并 develop：
  - feature branch `codex/bom-price-industry-polish-20260603` 推送提交 `6ef54131`。
  - `develop` fast-forward 到 `6ef5413121796d4b5a732dffa5193ac0c7b2ba23` 并推送。
- development 部署：
  - `./deploy_orderapp.sh development` 通过，Docker build 内部 `go test ./...` 通过。
  - 备份路径：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603210236`。
- smoke：
  - `docker compose ps`：`erp_orderapp`、`erp_postgres`、`erp_caddy`、`erp_docconvert` 运行中，Postgres healthy。
  - 未认证 `/app/` 返回 `303` 到 `/app/orders`。
  - BasicAuth `/app/vue-shell` 返回 `200`。
  - 需求 API 返回 `200` 且包含 `PR-403-BOM-PRICE-INDUSTRY-POLISH`。
  - BOM list API 返回 `200`，`Codex测试豆 production_bom_id=604 version=V001 group=0`。

## 手册
- `OP_MANUAL_COSTING.md`：商品价格表提示改为“未设置计价方式”，并写清 `商品与配方 → 商品配置和分类模板 → 商品配置模板 → 计价方式`。
- `OP_MANUAL_INVENTORY_MATERIALS.md`：补充生产 BOM 共同操作区、BOM 列表滚动窗口、旧 BOM 绑定修复和行业字段模板搜索筛选。
- `OP_MANUAL_PRODUCTION.md`：补充行业字段模板左列表搜索/过滤、右侧编辑的操作口径。
