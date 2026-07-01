# PR-513-MATERIAL-CLASSIFICATION-TEMPLATE

## Scope
- 物料档案分类改用系统 `分组模板`，交互参考商品档案。
- 物料页通过 `BusinessGroupControls` 选择模板、目标分类和 `移动到分类`。
- 物料归组写入通用 `/api/business-group-assignments`，语义为 `usage_key=material_catalog`、`object_key=material`、`object_id=materials.id`。
- 旧 `material_classification_*` 数据只作为迁移来源，上线后迁移到 `物料档案默认分组`，保留历史大类、小类和物料归属。

## Verification
- RED: `node --test src/lib/materials-ui.test.js` 先失败，因为 `MaterialsView.vue` 仍使用本地分类 Tab、`增加分类` 和 `material-classification` 写接口。
- RED: `go test ./internal/infrastructure/postgres/catalog -run TestMaterialClassificationMigratesToBusinessGroupAssignments -count=1` 先失败，因为 catalog schema 未迁移旧物料分类。
- RED: `go test ./internal/interfaces/http/support -run TestDev513MaterialClassificationTemplateContracts -count=1` 先失败，因为 PR-513 docs/seed 标记缺失。
- GREEN: 前端、后端和支持层测试见最终交付记录。

## Browser Acceptance
- 打开 `productPriceManagement` 不受影响；本需求浏览器重点为 `vue-shell?view=materials`。
- 物料列表顶部展示 `分组模板` 控件，不再展示旧 `全部分类 / 未分类 / 增加分类` Tab 区。
- 选择模板后，物料按模板大类、小类和 `未分类` 展示，空分类可见。
- 勾选物料并点击 `移动到分类` 后，页面无重叠、无控制台错误，刷新后分类保持。
