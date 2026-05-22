# PR-315-FORM-DRAFT-CACHE 验收记录

## 范围
- 录单、BOM配方维护、SKU设置 的未提交表单在本次浏览器会话内保留。
- 跳转到其他功能再回来时保留未提交表单。
- 刷新浏览器后草稿清空。

## 证据
- 前端单测：`node --test src/lib/view-routing.test.js src/lib/form-draft-cache.test.js`
- 支持层测试：`go test ./internal/interfaces/http/support -run 'TestDev31[45]' -count=1`
- 手册：`OP_MANUAL_ORDER_SALES.md`、`OP_MANUAL_INVENTORY_MATERIALS.md`

## 验收步骤
1. 在录单新建表单填写客户、商品明细和备注，不保存；切到其他功能后再回录单，确认未提交内容仍在。
2. 在 BOM配方维护 选择 SKU、填写组件或版本备注，不保存；切到其他功能后再回 BOM，确认内容仍在。
3. 在 SKU设置 填写公共产品、客户专属 SKU 或梯度模板草稿，不保存；切到其他功能后再回 SKU设置，确认内容仍在。
4. 刷新浏览器，再进入上述页面，确认临时草稿清空。
