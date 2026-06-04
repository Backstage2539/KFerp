# PR-396 商品档案复制、分类交互与产品价格表修正

## 范围
- 商品档案复制改为复制商品档案配置，并下线历史 SKU 复制入口和 API。
- 商品档案、客户商品分类交互改为“增加分类 / 移动到分类 / 移动到子类”，未分类有独立 Tab，移动归类直接覆盖旧归类。
- 客户商品编号由后端自动生成，单个新增和批量添加商品档案不手工填写。
- 产品价格表候选从当前分类 assignment 读取，未归类显示“其他”。

## 验证方式
Van 本轮要求不做浏览器/人工验收；本记录只保留代码、单测、API 测试、构建和 changed verifier 证据。

## 验收点
- 商品档案页无卡片创建入口、无历史 SKU 复制入口；复制为商品档案调用新 product copy API。
- 商品状态只显示启用/停用；BOM 被启用商品引用时失效接口拒绝。
- 商品档案和客户商品都有未分类 Tab；全部/未分类 Tab 使用“移动到分类”，分类模板 Tab 使用“移动到子类”。
- 客户商品编号输入框不在单个新增和批量新增中出现，后端自动生成编号。
- 产品价格表商品类型来自分类模板；未归类进入“其他”；发布快照保留分类模板和分类项字段。

## 测试证据
- `node --test orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/product-bean-list-split.test.js`
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/domain/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support`

## 手册证据
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
