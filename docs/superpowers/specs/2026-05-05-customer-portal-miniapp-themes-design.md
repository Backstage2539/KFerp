# 客户门户小程序主题配置设计

日期：2026-05-05

## 背景

客户服务小程序已经具备登录、客户首页、我的豆单、我的订单、现货/代发/代加工/库存/物流/结算等入口。当前页面视觉以灰底、白卡、黑色按钮为主，能用但显得简陋，缺少咖啡源头工厂的品牌质感，也不便于针对不同客户展示不同门户气质。

Van 已确认：三套样式方向都保留，但小程序用户不在手机端自行切换；主题由 ERP 的客户门户配置按客户选择。

## 目标

- ERP 客户门户配置页可为每个客户选择一套小程序主题。
- 小程序登录后按当前客户的主题渲染首页、服务页、订单、表单和豆单外层体验。
- 第一版提供三套内置主题：
  - `coffee_factory`: 咖啡工厂专业风，默认主题，暖咖啡色，品牌感强。
  - `clean_ops`: 清爽业务工具风，克制清楚，适合高频订单/物流查询。
  - `premium_partner`: 品牌会员高级风，高质感，适合对外展示和合作伙伴入口。
- 旧客户或未配置主题的客户自动使用 `coffee_factory`，不影响现有登录和能力配置。

## 非目标

- 第一版不开放自定义颜色、字体、背景图上传或用户端切换，避免客户配置出不可读界面。
- 第一版不重做业务流程、接口权限或服务能力模型。
- 第一版不把主题塞进单个能力的 `config_json`，因为主题是客户门户级属性，不属于某个服务能力。

## 推荐方案

在 `customer_portal_profiles` 增加 `theme_key` 字段，作为客户门户级配置。后端在客户上下文和服务页 API 中返回主题；小程序用统一主题 token 映射样式。

选择这个方案的理由：

- 主题和 `display_name`、`enabled` 同属客户门户 profile，数据归属清晰。
- 保存和读取链路和现有门户配置一致，改动集中。
- 小程序端只需要读取当前客户上下文，不需要理解后台能力配置细节。
- 未来如果增加全局默认主题或品牌包，可以在 profile 上继续扩展。

## 数据模型

`customer_portal_profiles` 新增：

- `theme_key TEXT NOT NULL DEFAULT 'coffee_factory'`

合法值：

- `coffee_factory`
- `clean_ops`
- `premium_partner`

后端必须归一化非法或空值为 `coffee_factory`。数据库可以只做默认值，不强依赖 check constraint；应用层负责兼容旧数据和未来扩展。

## API 合同

客户上下文返回新增字段：

- `theme_key`

影响接口：

- `POST /api/mini/login`
- `GET /api/mini/me`
- `POST /api/mini/current-customer`
- `GET /api/mini/services/:key`

后台配置接口：

- `GET /api/customer-portal/admin/customers`
- `GET /api/customer-portal/admin/customers/:id`
- `PUT /api/customer-portal/admin/customers/:id/visibility`

`PUT /visibility` 请求新增：

```json
{
  "display_name": "浅焙作坊咖啡",
  "enabled": true,
  "theme_key": "premium_partner",
  "capabilities": []
}
```

响应中的 `customer` 同步返回 `theme_key`，便于保存后刷新当前行。

## ERP 页面设计

在 `CustomerPortalSettingsView` 的每个客户配置行增加“门户主题”单选区域。

交互：

- 显示三张主题卡片：咖啡工厂专业风、清爽业务工具风、品牌会员高级风。
- 每张卡显示色块、名称和一句用途说明。
- 只能选择一套主题。
- 保存配置时与显示名、启停、能力开关一起提交。
- 保存成功后当前行保持选中主题。

页面布局仍保留当前列表行内编辑模式，避免回到旧的底部详情面板。

## 小程序设计

小程序新增主题定义模块，例如 `src/utils/themes.ts`：

- `normalizeThemeKey(value)`：非法值归一到 `coffee_factory`。
- `miniappThemeClass(themeKey)`：返回页面根节点 class。
- `themeMeta(themeKey)`：返回显示文案、强调色、按钮色、卡片背景、弱文本色等 token。

页面使用方式：

- 登录后 session 保存 `theme_key`。
- `home.vue`、`service.vue` 根节点增加主题 class。
- 登录页未登录时使用默认 `coffee_factory`。
- 服务页从 `page.theme_key` 或 session 读取主题，切换客户后立即跟随新客户主题。

主题影响范围：

- 页面背景。
- 顶部 hero。
- 服务入口卡片。
- 指标卡。
- 按钮、筛选 chip、picker、输入框。
- 订单/库存/费用列表卡。
- 豆单外层容器和缓存提示。豆单内部发布内容仍尊重 ERP 豆单发布配置的背景色、字体色、logo 和分组样式。

## 三套主题定义

### 咖啡工厂专业风

- 默认主题。
- 适合大多数客户。
- 暖咖啡色 hero、米色页面背景、白色或浅米卡片。
- 强调“源头工厂服务”和咖啡品类识别。

### 清爽业务工具风

- 适合高频查订单、物流、库存的客户。
- 浅灰绿背景、白色卡片、绿色/深灰强调色。
- 信息密度最高，装饰最少。

### 品牌会员高级风

- 适合对外展示、合作伙伴、重点客户。
- 深色金棕 hero、暖白背景、金色强调。
- 视觉质感最强，但业务列表页要控制装饰，保持可读。

## 错误处理

- 后端收到空或未知 `theme_key` 时保存为 `coffee_factory`。
- 小程序收到未知主题时使用 `coffee_factory`。
- 后台配置页加载旧数据没有 `theme_key` 时选中默认主题。
- 保存失败时保留当前表单选择并展示错误，不静默回退。

## 测试计划

单元测试：

- 后端主题归一化：空值、未知值、三种合法值。
- `customer_portal_profiles` schema 源码包含 `theme_key` 默认值。
- 客户门户服务返回 `CurrentContext` 和 `ServicePage` 的主题。
- 小程序主题 helper 覆盖三种主题和非法值回退。
- 小程序页面源码守卫：首页和服务页根节点应用主题 class。
- 需求种子包含 PR/DEV/UT/API/REV。

API 测试：

- 后台 `PUT /api/customer-portal/admin/customers/:id/visibility` 可保存 `theme_key` 并在详情返回。
- `GET /api/customer-portal/admin/customers/:id` 返回客户主题。
- `/api/mini/me` 返回当前客户主题。
- `/api/mini/services/:key` 返回当前客户主题。

构建验证：

- `npm test --prefix miniapp`
- `npm run typecheck --prefix miniapp`
- `npm run build:mp-weixin --prefix miniapp`
- Go 相关包测试和全量 `go test ./... -count=1`

## 手册与验收

需要更新：

- `orderapp-remote/docs/customer-portal-miniapp-test.md`：增加主题选择与小程序预览验收。
- `REQUIREMENTS.md` 和 `orderapp-remote/docs/REQUIREMENTS.md`：新增客户门户主题配置需求。
- `ACCEPTANCE_TESTS.md` 和 `orderapp-remote/docs/ACCEPTANCE_TESTS.md`：新增三套主题可选、按客户生效、默认回退的验收项。
- 五张需求表种子：PR、DEV、UT、API、REV。

验收口径：

- ERP 客户门户配置页能为同一客户保存三套任意主题之一。
- 小程序登录该客户后首页和服务页跟随所选主题。
- 未配置主题客户默认显示咖啡工厂专业风。
- 切换客户时主题跟随当前客户变化。
- 旧能力配置和豆单原生展示不被破坏。
