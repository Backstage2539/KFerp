# PR-438-PRODUCT-TEMPLATE-DELETE-PRICE-LIST-UI

## 范围
- 已发布价格表支持搜索、分页展示和收起/展开。
- 全局单位字典、单位模板、分类模板和商品配置模板删除后不再展示，删除不等于失效。
- 生产 BOM 选择产出商品时过滤已失效商品。
- 阶梯价模板展示单位来自系统全局单位字典，不再依赖单位模板。
- 商品配置模板支持删除；停用商品配置模板时不再因历史单位模板 inactive 报 `unit template inactive`。

## RED 证据
- `node --test src/lib/product-price-list-types.test.js src/lib/product-settings.test.js src/lib/bom.test.js`：新增前缺少 `publicationVersionListState` 和 `visibleNonDeletedRows`。
- `go test ./internal/interfaces/http/catalog -run 'TestProductSettingsAPIDeletesProductConfigTemplate|TestProductSettingsAPIDeletesGlobalUnitsAndUnitTemplates' -count=1`：新增前缺少 `DeleteProductConfigTemplateCommand`。
- `go test ./internal/infrastructure/postgres/catalog -run 'TestTemplateDeletesUseDeletedStateAndHideFromLists|TestProductUnitDeletesSoftDisableAndAudit' -count=1`：新增前缺少删除态字段和隐藏查询。
- `go test ./internal/interfaces/http/support -run TestDev438ProductTemplateDeletePriceListUI -count=1`：新增前缺少 PR-438 文档和验收标记。

## 验收项
- [x] 已发布价格表可搜索、分页和收起/展开。
- [x] 删除后的全局单位字典、单位模板、分类模板和商品配置模板不再出现在列表和新建/编辑候选。
- [x] 生产 BOM 产出商品选择器不展示已失效商品。
- [x] 阶梯价模板展示单位读取全局单位字典。
- [x] 商品配置模板可删除；停用历史单位模板已 inactive 的商品配置模板不再报 `unit template inactive`。

## 后续证据
- GREEN frontend: `node --test src/lib/product-price-list-types.test.js src/lib/product-settings.test.js src/lib/bom.test.js` passed 132/132.
- GREEN targeted API/repository/support: catalog API, catalog repository and support PR-438 tests passed.
- GREEN broader: `npm run build` in `frontend-vue-shell`; `go test ./...` in `orderapp-remote`; `scripts/verify_kferp.sh changed`; `git diff --check`.
- GREEN merge/deploy: feature branch pushed; `develop` fast-forwarded to `580c670d6b6dffcb5769bc63e6ad99d03a24c6d3` and deployed to development with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260606220848`.
- GREEN smoke: Docker build ran `go test ./...`; `erp_orderapp` restarted; unauthenticated `/app/` returned 303; authenticated `/app/vue-shell/?view=costing` returned 200; requirement API exposes `PR-438-PRODUCT-TEMPLATE-DELETE-PRICE-LIST-UI`; `/api/bom/products` returned `inactive_rows=0`.
- GREEN browser: 商品价格表已发布价格表搜索 `V3.0.7` 后显示 `1 / 19` and `共 1 条 / 1 页`; click `收起` hid rows and showed `展开`; 商品配置模板 edit state shows `停用配置` and `删除配置`; 阶梯价模板 visible labels include `展示单位` and no visible `单位模板`/`引用模板` selector.
