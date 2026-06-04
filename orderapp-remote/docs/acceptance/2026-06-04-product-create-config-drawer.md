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

## 部署证据
- 运行时代码提交：`471a2df3 fix product create config drawer`
- 需求种子提交：`57f660d9 test: seed product create config drawer requirement`
- 合并：以上提交已快进合入并推送 `origin/develop=57f660d94ff9c905d3c2beb2d3f2d6eee349b27e`。
- 部署：`./deploy_orderapp.sh development`。
- 备份：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604175632`。
- 部署构建：Docker build 期间 `go test ./...` 通过。
- Smoke：`erp_orderapp`、`erp_postgres`、`erp_caddy`、`erp_docconvert` running；未认证 GET `/app/` 返回 303；认证 `/app/vue-shell` 返回 200；认证 `/app/api/product-settings` 返回 200；需求 API 暴露 `PR-408-PRODUCT-CREATE-CONFIG-DRAWER`；远端源码包含 `resolveCreatedProductForConfig`。
