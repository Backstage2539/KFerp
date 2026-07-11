# PR-528 分组模板与系统设置页面分离

## 验收目标

- `设置 / 分组模板` 是独立 Vue 页面，只维护模板、模板大类和小类。
- `设置 / 系统设置` 只维护系统开关和全局单位字典，不加载分组模板数据。
- `view=groupTemplates` 和旧 `view=groupManagement` 均进入分组模板页；`view=uiSettings` 进入系统设置页。
- 拆分只改变前端页面边界，不迁移或修改现有模板、分类、商品、BOM、仓库及其归类关系。

## TDD 证据

- RED：`node --test src/lib/group-settings-separation.test.js` 因缺少 `GroupTemplatesView.vue` 失败。
- GREEN：`node --test src/lib/group-settings-separation.test.js src/lib/materials-ui.test.js src/lib/product-bean-list-split.test.js src/lib/menu-ia.test.js src/lib/product-settings.test.js` 通过 196/196。
- 文档契约 RED：`go test ./internal/interfaces/http/support -run TestDev528GroupSettingsSeparationContracts -count=1` 因缺少 PR-528 需求标记失败。
- 文档契约 GREEN：`go test ./internal/interfaces/http/support -count=1` 通过；旧 PR-440/453/455/506/513 契约同步改为独立分组模板入口。
- 构建 GREEN：`npm run build` 通过。
- 仓库校验 GREEN：`scripts/verify_kferp.sh changed` 与 `git diff --check` 通过。
- 部署后浏览器证据在 development / production 发布后补齐。

## 浏览器验收

- 开发环境分别打开 `view=groupTemplates`、`view=uiSettings` 和旧 `view=groupManagement`，确认标题、字段和接口边界正确。
- 生产环境重复同一只读验收，确认正式入口与开发环境一致。
- 浏览器验收不新增、编辑或删除真实模板和全局单位。
