# PR-416 物料档案列表布局与批量失效

## 范围
- 物料档案列表删除 `物料类型` 列，分类通过 Tab 和分组体现。
- 搜索、状态过滤、新建物料、批量失效移动到 `物料列表` 上方。
- 表格最左侧支持当前列表/分组全选，勾选后执行批量失效。
- 物料列表提供横向滚动和更稳定列宽，避免窄面板下行内容挤压。

## 验收点
- 物料列表不展示 `物料类型` 列；未分类物料进入 `未分类` 视图或分组。
- 物料列表工具栏包含搜索、状态、查询、新建物料和 `批量失效`。
- 表头复选框可全选或取消全选当前列表/分组；批量失效逐条调用现有物料失效接口并保留操作日志。
- 列表横向滚动可查看宽内容，物料名称、单位、库存数量和状态不再挤在一起。

## 证据
- RED: `node --test src/lib/materials-ui.test.js` 在旧实现缺少 `批量失效`、列表工具栏和全选时失败。
- RED: `go test ./internal/interfaces/http/materials -run 'TestMaterialsViewUsesClassificationAndIndustryFields|TestMaterialsViewListLayoutSupportsBulkSelection' -count=1` 在旧实现缺少 `deprecateSelectedMaterials` 和 `material-list-toolbar` 时失败。
- GREEN: `node --test src/lib/materials-ui.test.js` passed 4/4。
- GREEN: `node --test src/lib/materials-ui.test.js src/lib/menu-ia.test.js` passed 18/18。
- GREEN: `go test ./internal/interfaces/http/materials ./internal/application/materials ./internal/infrastructure/postgres/materials ./internal/interfaces/http/support -count=1` passed。
- GREEN: `npm run build` passed with existing chunk-size warning。
- GREEN: `scripts/verify_kferp.sh changed` exited 0。
- Deploy: development stack deployed at `origin/develop=b9bf889d8312f55b45b2c3f2eb19cb5783f15bbe`，backup `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260605013954`。
- Smoke: `/app/vue-shell?view=materials` authenticated GET 200，`/app/api/materials?limit=5` authenticated GET 200，需求接口返回 `PR-416-MATERIALS-LIST-LAYOUT-BATCH-DEPRECATE`。
- Browser: 物料档案页实际 DOM 表头为 `物料名称 / 单位 / 库存数量 / 状态`，没有 `物料类型`；存在 `批量失效`、`全选物料`；表格 `min-width` 生效为 `920px`，容器 `overflow-x=auto`。
