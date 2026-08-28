# PR-615 价格试算提速、展示统一与模板快捷编辑验收记录

## 范围

- 批量价格试算按商品范围加载，并去重生产选项与成本上下文。
- 价格表预览、PDF 和公开页使用同一整数/两位小数展示规则。
- 平铺价格行新增复用式价格模板编辑抽屉，保存后定向重算当前草稿。
- 只交付 development；不操作 `main`、production 或生产业务数据。

## 自动验证

- RED：范围加载接口缺失；同商品四档重复加载生产选项和成本明细；价格 `38.2` 被取整为 `38`；模板编辑按钮、共享表单和定向缓存失效缺失。
- GREEN：见 `internal/application/costing/service_test.go`、`internal/infrastructure/postgres/costing/repository_test.go`、`internal/interfaces/http/costing/costing_api_test.go`、`frontend-vue-shell/src/lib/bean-list-pdf.test.js`、`costing-price-list-workflow.test.js` 和 `costing-bean-list-version-ui.test.js`。
- GREEN：`scripts/verify_kferp.sh all` 通过；Go 全量、Vue 1043/1043 和 Vite 6596 modules 构建均通过，`git diff --check` 通过。
- 开发预检、合并提交、开发部署健康与真实四档耗时在发布阶段补录。

## 人工验收边界

- Van 在 development 商品价格表选择一个包含四个档位的商品，首次生成后确认两秒内出现全部价格。
- 编辑当前引用模板并保存，确认仅相关自动行进入“价格计算中”，人工价保留，重算完成后可撤销人工修改到新基准。
- 确认预览、生成 PDF 与公开页的小数展示一致。
