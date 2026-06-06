# PR-428 商品档案去掉生豆属性和绑定熟豆验收记录

## 范围
- 商品档案列表中的生豆 SKU 不再展示“生豆属性”或“绑定熟豆”编辑控件。
- 新增、客户商品新增和基础信息保存 payload 不再提交 `green_bean_type` 或 `green_bean_bom_product_id`。
- 前端不再维护单品/拼配硬编码枚举或绑定熟豆候选；旧后端字段保留历史兼容，不做破坏性删除。

## 验收口径
- 打开 商品档案，列表行详情不出现“生豆属性”“绑定熟豆”。
- 新增生豆商品档案或保存生豆基础信息时，只提交名称、备注、商品类型/分类和商品配置相关字段，不提交硬编码生豆枚举。
- 手册说明当前商品档案列表不再维护这两个字段；需要表达生豆业务属性时，使用商品分类、商品配置或商品档案配置抽屉中的行业字段。

## 自动化证据
- RED：`node --test src/lib/product-settings.test.js` 在修复前失败，原因是 payload 仍包含 `green_bean_type` / `green_bean_bom_product_id` 且 Vue 视图仍暴露移除目标控件。
- GREEN：`node --test src/lib/product-settings.test.js` 通过 108/108。
- GREEN：`go test ./internal/interfaces/http/support -run 'TestProductArchiveNoGreenBeanBindingRequirementSeedsExist|TestProductArchiveNoLongerExposesGreenBeanBindingEditors|TestGreenBeanSalesWiringAndManuals' -count=1`。
- GREEN：`go test ./internal/interfaces/http/support ./internal/interfaces/http/catalog -run 'TestProductArchiveNoGreenBeanBindingRequirementSeedsExist|TestProductArchiveNoLongerExposesGreenBeanBindingEditors|TestGreenBeanSalesWiringAndManuals|TestProductSettingsAPI' -count=1`。
- GREEN：`go test ./...` in `orderapp-remote`。
- GREEN：`npm run build` in `orderapp-remote/frontend-vue-shell` passed with the existing Vite chunk-size warning.
- 待发布后补充：development 浏览器验收 商品档案。

## 手册
- `OP_MANUAL_GREEN_BEAN_SALES.md`
- `OP_MANUAL_INVENTORY_MATERIALS.md`
- `OP_MANUAL_CUSTOMER_PORTAL.md`
- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
