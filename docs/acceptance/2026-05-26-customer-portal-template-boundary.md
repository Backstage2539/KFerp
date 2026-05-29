# PR-381-CUSTOMER-PORTAL-TEMPLATE-BOUNDARY 验收记录

## 需求
- 客户档案只维护客户类型、基础资料和“开通客户门户/工作台”开关。
- 能力模板只在门户客户配置中绑定、校验和变更。
- 已开通但未绑定有效能力模板的客户可以进入门户配置默认列表，但不能进入履约客户候选、客户工作台入口或客户侧下单能力。

## 验收项
- [ ] 客户档案新增/编辑客户时，打开“开通客户门户/工作台”不要求能力模板。
- [ ] 客户档案和录单客户抽屉不显示能力模板下拉，不自动推荐模板。
- [ ] 客户保存时仍写入 `portal_enabled` 的操作日志。
- [ ] 门户客户配置页仍显示能力模板下拉，并保留未知模板提示和模板有效性校验。
- [ ] 未绑定有效模板的已开通客户不出现在履约客户候选或客户工作台入口。

## 证据
- 单元/API：`go test ./internal/application/customer ./internal/interfaces/http/customer ./internal/infrastructure/postgres/customer ./internal/interfaces/http/support -run 'TestServiceAllowsPortalSwitchWithoutCapabilityTemplate|TestCustomerAPISupportsChannelPortalSwitchWithoutTemplateBinding|TestCustomerUpsertPersistsPortalSwitchWithoutBindingTemplate|TestDev380' -count=1`
- 前端：`node --test src/lib/customer-types.test.js src/lib/order-entry.test.js`
- 浏览器：客户档案打开门户/工作台开关、门户客户配置绑定模板、履约候选过滤流程。
