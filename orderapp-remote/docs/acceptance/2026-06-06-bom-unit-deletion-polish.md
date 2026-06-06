# PR-436-BOM-UNIT-DELETION-POLISH 验收记录

## 范围
- 生产 BOM 支持自定义大组和组内小分类删除。
- `移动到分组`、`移动到小分类` 按钮放在对应目标选择器左侧。
- BOM 产出商品和商品组件新候选过滤失效商品。
- 单位模板支持删除。
- 全局单位字典支持删除。

## RED
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js`：实现前失败于 BOM 产出商品仍包含失效商品、单位模板/全局单位字典缺少删除入口。
- `go test ./internal/interfaces/http/catalog -run 'TestProductSettingsAPIDeletesGlobalUnitsAndUnitTemplates|TestProductSettingsAPISupportsGlobalUnitDefinitionsAndTemplates' -count=1`：实现前失败于缺少单位模板和全局单位字典删除命令。
- `go test ./internal/infrastructure/postgres/catalog -run TestProductUnitDeletesSoftDisableAndAudit -count=1`：实现前失败于缺少软删除和审计持久化。
- `go test ./internal/interfaces/http/support -run TestDev436BomUnitDeletionPolish -count=1`：实现前失败于缺少 PR-436 需求种子、源码标记和文档标记。

## GREEN
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js`：124/124 passed。
- `go test ./internal/interfaces/http/catalog -run 'TestProductSettingsAPIDeletesGlobalUnitsAndUnitTemplates|TestProductSettingsAPISupportsGlobalUnitDefinitionsAndTemplates' -count=1`：passed。
- `go test ./internal/infrastructure/postgres/catalog -run TestProductUnitDeletesSoftDisableAndAudit -count=1`：passed。
- `go test ./internal/interfaces/http/support -run TestDev436BomUnitDeletionPolish -count=1`：passed。
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/support -count=1`：passed。
- `go test ./...`：passed。
- `npm run build` in `orderapp-remote/frontend-vue-shell`：passed；保留现有 large chunk/plugin timing warning。
- `scripts/verify_kferp.sh changed`：passed。
- `git diff --check`：passed。
- Browser harness acceptance：用真实 Vue SFC 和只读 mock API 挂载 生产 BOM、单位模板、全局设置；确认 `移动到小分类` 位于 `目标小分类` 左侧，大组和小分类删除入口可见，失效商品不出现在新建 BOM 产出候选，单位模板删除按钮可见，SKU 设置抽屉和全局设置页都能在编辑基础单位时看到删除按钮。
- Development deploy: `./deploy_orderapp.sh` deployed `origin/develop=a79262b06ba9481180bd3684d94c988ef01d899d` to the development stack. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260606202820`.
- Deployment smoke: Docker build ran `go test ./...`; `erp_orderapp` restarted and is running; unauthenticated `/app/` returned 303; authenticated `/app/vue-shell/` returned 200; requirement API exposes `PR-436-BOM-UNIT-DELETION-POLISH`; deployed source contains `delete_product_unit_template` and PR-436 docs/source markers.

## 手册与验收口径
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`：补充 BOM 分组删除、移动按钮顺序、失效商品过滤和单位模板删除。
- `orderapp-remote/docs/OP_MANUAL_SETTINGS_AUDIT.md`：补充全局单位字典删除和历史引用不回改。
- `orderapp-remote/docs/REQUIREMENTS.md` 与 `orderapp-remote/docs/ACCEPTANCE_TESTS.md`：登记 PR-436 需求和 K41 验收项。
