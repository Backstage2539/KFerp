# PR-529 分组模板分类删除

## 需求

- 分组模板的大类、小类统一使用删除，不再提供分类启用/停用。
- 删除小类只删除该小类；删除大类递归删除该大类和全部小类。
- 商品、生产 BOM、仓库、物料等引用受影响分类的对象自动归入当前模板 `未分类`，业务对象和历史快照不删除。

## 实现

- Vue 页面为大类和小类提供删除入口，删除确认区分大类/小类影响范围，并移除分类启用开关和全部停用文案。
- `DELETE /api/business-group-items/:id` 在同一事务内递归查找目标分类树，先把 `business_group_assignments.group_item_id` 更新为 `0`，再物理删除分类行。
- 操作日志动作保持 `delete_business_group_item`，记录分类类型、删除分类 ID、删除数量和自动归入未分类的引用数量。

## TDD 证据

- RED frontend：`node --test src/lib/group-template-category-delete.test.js` 因页面仍包含分类启用开关、停用确认和停用结果，且小类没有删除入口而失败。
- RED repository：`go test ./internal/infrastructure/postgres/catalog -run TestDeleteBusinessGroupItemPhysicallyDeletesTreeAndUncategorizesAssignments -count=1` 因仓储仍把分类设为 `active=false`、没有物理删除而失败。
- RED support：`go test ./internal/interfaces/http/support -run TestDev529GroupTemplateCategory -count=1` 因 PR/DEV、文档和删除合同标记缺失而失败。
- GREEN frontend：`node --test src/lib/group-template-category-delete.test.js src/lib/group-settings-separation.test.js src/lib/business-grouping.test.js src/lib/product-settings.test.js src/lib/bom.test.js src/lib/materials-ui.test.js` 通过 189/189。
- GREEN repository/API：`go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/support -count=1` 通过。
- GREEN build/backend：`npm run build`、`scripts/verify_kferp.sh changed`、`scripts/verify_kferp.sh backend`、`git diff --check` 通过。

## 验收

- [ ] 大类、小类均可删除，页面没有分类启用/停用交互。
- [ ] 删除小类后，引用该小类的对象进入当前模板 `未分类`。
- [ ] 删除大类后，大类和全部小类消失，引用其中任一分类的对象进入当前模板 `未分类`。
- [ ] 操作日志可查，商品、BOM、仓库、物料、库存及历史快照不被删除。

## Development Deploy

- `develop` 应用提交：`2ad8c14c3d1435461ed446cf20c367bd2b95f621`。
- 发布命令：`./deploy_orderapp.sh development`；备份：`/opt/stacks/erp/orderapp.backup.deploy-20260712135112`。
- 容器、认证入口、PR-529 需求标记、部署源码和近 5 分钟错误日志检查通过。
- 真实 API/数据库自清理场景通过：删除小类后分类行消失且引用 `group_item_id=0`；删除大类后大类和子类同时消失，两个引用均归零；两条 `delete_business_group_item` 日志存在，`assignments_uncategorized` 合计为 3；临时模板、分类、用途和归类全部清理。
- 开发浏览器页面显示大类/小类删除入口，分类编辑区没有启用复选框或启用/停用文案，控制台错误 0；界面验收临时模板已清理。
