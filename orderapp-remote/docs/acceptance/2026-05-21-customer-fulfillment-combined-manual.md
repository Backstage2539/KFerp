# 验收记录：客户履约三份手册合并

## 需求
- 将客户履约下的“工作台模式手册、客户门户手册、履约操作手册”归纳为一个综合手册。
- 更多使用图示展示整体路径、角色权限和功能流程。
- 区分不同角色的操作权限和不同功能的操作流程，便于新用户上手。

## 文档和入口变更
- `OP_MANUAL_CUSTOMER_FULFILLMENT.md` 改为“客户履约全流程”，合并工作台模式、客户门户和履约账户内容。
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md` 与根目录手册保持一致。
- `OPERATION_MANUALS.md` 和 `orderapp-remote/docs/OPERATION_MANUALS.md` 将客户履约手册索引收口到一个综合手册。
- `orderapp-remote/frontend-vue-shell/src/lib/menu-ia.js` 的客户履约菜单只展示一个“客户履约手册”入口。
- 旧 `workspaceModeManual`、`customerPortalManual` view key 继续作为隐藏兼容入口，统一打开 `OP_MANUAL_CUSTOMER_FULFILLMENT.md`。

## 覆盖内容
- 新增“合并后的阅读地图”，说明客户档案、能力模板、外部用户、客户商品、下单/导入、订单履约、费用月结和操作日志的关系。
- 新增“角色权限图”，区分管理员、客户成功、履约运营、仓库/生产、财务、履约客户外部用户和零售商城客户的边界。
- 新增客户开通、工作台与客户上下文、订单履约、费用月结等流程图。
- 保留原有 Excel 导入、托管库存、代发/代加工、费用月结、常见问题和结果校验细节。

## 验证
- [x] 先改测试并确认旧实现失败：`node --test src/lib/menu-ia.test.js src/lib/operation-manuals.test.js`
- [x] 前端菜单、手册映射和工作台模式测试通过：`node --test src/lib/menu-ia.test.js src/lib/operation-manuals.test.js src/lib/workspace-mode.test.js`
- [x] 综合手册 Markdown 可解析出 8 个 flowchart：`node --input-type=module -e "...parseManualMarkdown..."`
- [x] 手册治理和客户履约手册守卫通过：`go test ./internal/interfaces/http/support -run 'TestOperationManual|TestCustomerFulfillmentManual' -count=1`
- [x] 根目录和部署目录手册一致：`diff -u OP_MANUAL_CUSTOMER_FULFILLMENT.md orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- [x] 根目录和部署目录索引一致：`diff -u OPERATION_MANUALS.md orderapp-remote/docs/OPERATION_MANUALS.md`

## 结论
- 通过。客户履约菜单已由三份手册收口为一份综合手册，旧链接兼容，综合手册用图示覆盖角色权限和主要业务流程。
