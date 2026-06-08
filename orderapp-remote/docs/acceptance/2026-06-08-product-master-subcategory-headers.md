# PR-451 商品档案子类分组标题

## Scope
- 商品档案列表使用 `product_catalog` 业务分组展示时，父组和子组都作为分类标题。
- 子组标题独立成行、缩进展示，只显示子组名；完整父/子路径作为标题上下文。
- `目标分组` 支持选择父组和子组，移动候选不显示“商品分组 /”分组集名称前缀。
- 不新增 API、不新增数据库字段，继续复用 `business_group_assignments` 和现有商品移动接口。

## Local Evidence
- RED frontend：`node --test src/lib/product-settings.test.js` 在实现前失败，因为 `businessGroupItemMoveOptions(..., { includeGroupName: false })` 缺少子组 depth/parent 元数据，商品档案分组标题没有子组层级信息。
- GREEN frontend：`node --test src/lib/product-settings.test.js src/lib/bom.test.js` passed 137/137。
- Support contract：`go test ./internal/interfaces/http/support -run 'TestDev451ProductMasterSubcategoryHeadersContracts|TestDev450BomGroupUsageSelectionContracts' -count=1` passed。
- Broader checks：`go test ./internal/interfaces/http/support -count=1`、`go test ./...`、`npm run build`、`scripts/verify_kferp.sh changed`、`git diff --check` passed。Vue build 仅保留既有 chunk-size warning。

## Browser Acceptance
- 打开 development `商品档案`。
- 有父组商品时，分组标题显示父组名，例如 `商品-咖啡熟豆`。
- 有子组商品时，分组标题独立显示子组名，例如 `意式拼配豆`，并缩进显示。
- 分组标题上下文保留完整路径 `商品-咖啡熟豆 / 意式拼配豆`。
- `目标分组` 下拉包含父组和子组，可把商品移动到具体子类。
- 页面普通标签不显示 `商品分组 / 商品-咖啡熟豆 / 意式拼配豆` 这类分组集前缀。

## Deployment
- Feature branch pushed and merged to `develop`.
- Implementation commit `origin/develop=f4cebbeb6a00ba20736c2293c6ce5f3cdf7083b9` deployed to development.
- Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260608134526`.
- Smoke: containers running; `/app/vue-shell/?view=productMaster` authenticated 200; requirement API exposes `PR-451-PRODUCT-MASTER-SUBCATEGORY-HEADERS`; deployed docs/source expose PR-451 markers.
- Browser DOM acceptance: parent header `商品-咖啡熟豆` uses 16px indent; child header `意式拼配豆` uses `classification-subgroup-row`, full-path title `商品-咖啡熟豆 / 意式拼配豆`, and 40px indent; `目标分组` includes `商品-咖啡熟豆 / 意式拼配豆`; headers/options do not include `商品分组 /`; console errors 0.
- Note: in-app Browser screenshot capture timed out after DOM and console acceptance.
