# 客户价格表固定报价恢复

- PR-CUSTOMER-FIXED-QUOTE-RELOAD / DEV-CUSTOMER-FIXED-QUOTE-RELOAD。
- 客户选定固定价但没有公共或历史报价时，只要求填写该规格的客户价格；不得自动借用其他规格的价格或 0 元发布。
- 输入正价后，在命名表切换、草稿恢复和发布批次重算中保留金额与规格。已有客户报价的手工调整保持原来源，其他客户和其他规格不受影响。
- 旧草稿若已将报价迁移到精确规格的固定价设置，只有匹配商品和规格，或可唯一确认 BOM 规格身份且无历史报价时才恢复；已发布快照不回写。
- 验证：`node --test src/lib/customer-price-draft.test.js src/lib/price-table-editor-loading.test.js` 覆盖旧实现的 RED、修复后的 GREEN，以及模拟发布批次携带正确价格；价格表编辑、迁移和快照回归及发布接口测试另行执行。
- 操作说明：`docs/OP_MANUAL_COSTING.md`，由前端成本核价手册入口展示。真实客户报价、价格表发布与用户业务验收由 Van 完成。
