# PR-458 分组模板驱动业务列表整理

## Scope
- PR-458-GROUP-TEMPLATE-BUSINESS-LISTING：商品档案、生产 BOM、仓库库存统一选择 `分组模板` 后，业务列表按模板完整大类/小类树和 `未分类` 自动整理展示。
- 分组模板仍只在 `系统设置 / 分组模板` 维护模板名、大类、小类；模板内不维护商品、BOM、仓库对象。
- 商品、BOM、仓库移动分类继续写入 `business_group_assignments`，不新增表、不新增后端 API。

## Acceptance Checklist
- [x] 商品档案选择 `商品分组` 后，列表出现模板下全部大类/小类标题；`咖啡熟豆`、`挂耳咖啡` 等空大类也显示，未归类商品进入 `未分类`。
- [x] 商品档案没有分类过滤 Tab，商品表格没有独立 `分类` 列；分类归属只通过分组标题表达。
- [x] 商品档案可勾选商品，通过 `移动到分类` 移动到 `未分类`、大类或小类，保存覆盖旧归类并写操作日志。
- [x] 生产 BOM 选择分组模板后，BOM 列表按模板完整大类/小类和 `未分类` 展示，空分类也显示。
- [x] 生产 BOM 没有 `全部分类 / 未分类 / 分类项` 过滤 Tab；状态、搜索、批量失效和批量移动继续可用。
- [x] 仓库库存不再显示 `普通仓库`、`客户仓库` 固定分段；仓库按库存分组模板的大类/小类和 `未分类` 整理。
- [x] 仓库行可勾选，使用同一套 `移动到分类` 控件批量移动仓库；仓库类型和客户绑定只作为行内或抽屉信息保留。
- [x] 商品档案、生产 BOM、仓库库存三处页面共用 `BusinessGroupControls` 和 `business-grouping` helper。

## Verification
- RED frontend：`node --test orderapp-remote/frontend-vue-shell/src/lib/business-grouping.test.js orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/bom.test.js orderapp-remote/frontend-vue-shell/src/lib/materials-ui.test.js` 在实现前失败，因为共享 helper/control、模板空分类显示、商品/BOM 去 Tab 和仓库去固定分段 marker 缺失。
- GREEN frontend：`node --test orderapp-remote/frontend-vue-shell/src/lib/business-grouping.test.js orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/bom.test.js orderapp-remote/frontend-vue-shell/src/lib/materials-ui.test.js` 通过 151/151。
- GREEN support/API：`go test ./internal/interfaces/http/support -run 'TestDev45(3|5|6|7|8)' -count=1` 通过；`go test ./internal/interfaces/http/support -count=1` 通过。
- GREEN build/backend：`npm run build` in `orderapp-remote/frontend-vue-shell` 通过，保留既有 Vite chunk-size warning；`go test ./...` in `orderapp-remote` 通过；`scripts/verify_kferp.sh changed` 通过；`git diff --check` 通过。
- GREEN deploy：`./deploy_orderapp.sh` 已从 `origin/develop=dab12f2433c23eab9bfa9706d02d46a03f7baa5b` 部署到 development。备份：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260608195032`。部署过程完成 Vue shell build、miniapp typecheck/build、Docker build、容器内 `go test ./...`。
- GREEN smoke：`erp_orderapp` up、`erp_postgres` healthy；未认证 `/app/` 返回 `303` 到 `/app/orders`；认证访问 `/app/vue-shell/?view=productSettings&pr458=1`、`bom`、`warehouseInventory` 均返回 `200`；`/app/api/business-groups?usage_key=product_catalog` 返回 `200`；`/app/api/req/product?limit=500` 暴露 `PR-458-GROUP-TEMPLATE-BUSINESS-LISTING`。
- GREEN browser：部署后商品档案可见共享分组控件、`商品分组`、`移动到分类`、`咖啡熟豆`、`挂耳咖啡`、`未分类`，且无分类过滤 Tab、无独立 `分类` 列；生产 BOM 可见共享分组控件、模板分类树和 `移动到分类`，无 `使用分组`、无 `全部分类 / 未分类 / 分类项` Tab；仓库库存可见共享分组控件、模板分类树和 `移动到分类`，无 `普通仓库` / `客户仓库` 固定分段；三页控制台错误 0。浏览器验收只读确认移动目标，不改动线上业务归类；移动 payload、覆盖旧归类和共享逻辑由 frontend/helper/support 测试覆盖。
