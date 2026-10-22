# PR-716 运费配置优先级

- DEV-843：独立系统默认配置表、读写API、乐观版本与操作日志；报价统一按场景、客户普通、系统默认回退。
- DEV-844：物流设置取消客户选择；客户档案编辑抽屉提供普通运费；代发场景可继承；手册同步。
- RED：TestFreightTemplatePriorityAPI 返回系统默认 GET 404；Vue 默认编辑器无客户不能保存；代发继承校验失败。
- GREEN：隔离 PostgreSQL API/报价测试以及 Vue 编译后运行测试。完整门禁结果见交付记录。
- 普通录单、代发估算及提交共用解析方法；编辑器读原始配置，不把继承值复制进客户/代发配置。
- 明确包邮按所选层级生效；继承时附加运费须为0；已选无效模板或缺费率不回退。历史订单继续引用原政策与模板快照。
- 手册：`docs/OP_MANUAL_FREIGHT.md`，通过既有 Vue 运费手册入口展示。
- 开发环境部署：待门禁；生产未发布，业务验收待 Van。

## 验证记录
- `go test ./...` 全包通过（无数据库的全包门禁，数据库专项另行运行）。
- `ORDERAPP_TEST_DATABASE_URL=... go test ./internal/infrastructure/postgres/customerfulfillment -run 'Test(Freight|OrderFreight|Fulfillment.*Settings|Fulfillment.*Fee|LegacyEmployee)' -count=1` 通过，41.116s；独立测试 schema，结束后清理。
- `TestFreightTemplatePriorityAPI` 另含真实代发设置保存、继承报价及服务费保留，2.196s 通过。
- Vue 1508 项全部通过；生产构建通过。18 项运费/履约组件与纯函数回归通过。
- 本地浏览器核对两类编辑器：全局配置无客户选择器，客户配置可继承，字段和保存按钮完整可见；使用合成数据，不涉及真实业务验收。
