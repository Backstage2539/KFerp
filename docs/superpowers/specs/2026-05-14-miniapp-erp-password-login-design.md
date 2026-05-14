# 小程序 ERP 账号密码登录与个人中心设计

## 背景

当前客户小程序登录入口是微信 openid 登录。Van 不准备开放 openid 登录，希望客户自己输入 ERP 系统里的用户名/手机号和密码登录。现有小程序退出登录后，同一微信身份仍会回到原 openid 绑定客户，导致用户无法自然切换到另一个 ERP 客户账号。

本次目标是把小程序的用户入口改为 ERP 账号密码登录，并把退出、切换用户这类账号操作集中到个人中心。

## 产品范围

- 小程序登录页默认且唯一展示“用户名/手机号 + 密码”登录表单。
- 小程序前端不展示微信一键登录入口。
- 只允许 ERP 中的渠道客户账号登录客户小程序。
- 渠道客户账号必须已启用登录、未禁用，并且存在有效客户门户绑定。
- 内部员工账号不能登录客户小程序。
- 首页、商城页、服务页不再直接放“退出登录”按钮，改为进入“个人中心”。
- 个人中心展示当前客户信息，并提供“切换用户”和“退出登录”。
- 切换用户和退出登录都会清除本机小程序 token，并回到账号密码登录页。

## 不做内容

- 不把 ERP 后台内部 token 直接复用到小程序。
- 不给内部员工开放客户小程序入口。
- 不做客户自助注册、找回密码或短信验证码登录。
- 不改变现有客户门户能力、主题、首页模式、订单和商城业务接口的权限模型。
- 后端可保留旧微信登录接口用于历史兼容或联调，但小程序用户界面不再暴露它。

## 推荐方案

采用独立的小程序密码登录接口：

```text
POST /api/mini/login/password
{
  "login": "用户名或手机号",
  "password": "密码"
}
```

成功返回现有 `LoginResult` 结构：

```text
token, mini_user_id, current_customer_id, theme_key, miniapp_entry_mode, bindings, capabilities
```

这样小程序后续仍然走现有 `/api/mini/me`、`/api/mini/current-customer`、服务页和商城接口，不把内部 ERP 会话暴露到客户侧。

## 后端设计

### 账号校验

新增客户门户应用服务方法，例如 `LoginWithPassword`：

1. 接收 `login` 和 `password`。
2. 按 ERP 登录规则用手机号或员工名称查找 `company_employees`。
3. 要求员工记录满足：
   - `active=true`
   - `account_type='channel_customer'`
   - `employee_login_passwords.password_hash` 与 ERP 密码规则一致
   - `employee_login_passwords.login_disabled=false`
4. 要求该员工存在 `customer_erp_user_bindings.status='active'`。
5. 要求绑定客户 `customers.active=true`，且客户门户可用。
6. 找不到账号、密码错误、不是渠道客户、登录禁用、未绑定客户门户，都不返回敏感细节给前端。

### 小程序身份映射

密码登录仍创建小程序会话，而不是 ERP 内部会话。

- 使用稳定 openid 命名空间，例如 `erp-employee:<employee_id>`，在 `mini_users` 中建立或更新账号影子身份。
- 登录时根据 active 的 `customer_erp_user_bindings` 同步 `customer_portal_user_bindings`：
  - `mini_user_id`
  - `customer_id`
  - `role`
  - `status='approved'`
  - `approved_by='erp-password-login'`
- 然后复用现有 `CreateLoginSession`，让 token、当前客户、能力、主题和首页模式沿用原客户门户逻辑。

这样后续小程序 token 仍只代表客户小程序身份，不携带内部 ERP 权限。

### 错误映射

- 缺少账号或密码：`400 invalid request`
- 账号或密码错误：`401 invalid login`
- 渠道账号登录被禁用：`403 login disabled`
- 非渠道客户账号或无有效客户绑定：`403 customer binding not found`
- 小程序身份被停用：沿用 `403 mini user disabled`

前端文案保持简单：

- 账号或密码不正确
- 账号暂不可登录，请联系运营
- 账号未绑定客户，请联系运营

## 小程序设计

### 登录页

登录页改为账号密码表单：

- 账号输入框：用户名或手机号
- 密码输入框
- 登录按钮
- 错误提示

登录成功后：

- 保存 mini token
- 应用返回的客户上下文
- 按 `miniapp_entry_mode` 和能力跳转到服务首页或商城首页

### 个人中心

新增页面：

```text
pages/profile/profile
```

内容：

- 当前客户名称
- 当前账号/登录身份摘要（如果后端返回）
- 已绑定客户选择器（多个客户时显示）
- 切换用户
- 退出登录

“切换用户”和“退出登录”都清除本地 session 并跳到登录页。差异只体现在按钮文案，避免客户找不到切换账号入口。

### 页面入口调整

首页、商城页、服务页顶部只保留账号入口：

- `个人中心`
- 商城页仍保留 `我的订单`
- 多客户切换能力从业务页迁到个人中心，避免业务页顶部堆满账号操作

## 数据与兼容

- 旧微信 openid 登录接口不作为小程序 UI 入口。
- 已存在的 `mini_users`、`mini_sessions`、`customer_portal_user_bindings` 继续复用。
- ERP 渠道客户账号的密码仍由用户权限页设置和维护。
- 客户门户配置页仍负责绑定渠道客户账号到客户。

## 安全边界

- 小程序密码登录不创建 `login_sessions` 内部后台会话。
- 内部员工账号即使密码正确，也不能登录客户小程序。
- 客户数据访问仍由 mini token 当前客户和客户能力控制。
- 前端不传 `customer_id` 作为登录权限来源。
- 服务端同步绑定时只读取 active 的 ERP 客户绑定，不接受客户端声明绑定关系。

## 测试计划

### 单元测试

- 客户门户服务：渠道客户账号密码正确时返回 mini 登录结果。
- 客户门户服务：密码错误、内部员工、登录禁用、无 active 客户绑定均拒绝。
- Postgres 仓储：ERP 密码登录会创建或复用 `mini_users`，并同步 approved 小程序客户绑定。
- miniapp：API helper 暴露 `/api/mini/login/password`。
- miniapp：登录页源码不再展示微信一键登录，个人中心包含切换用户和退出登录。

### API 测试

- `POST /api/mini/login/password` 成功返回 token 和当前客户上下文。
- 错误密码返回 401。
- 内部员工返回 403。
- 未绑定客户门户的渠道账号返回 403。
- 登录成功后的 `/api/mini/me` 能读取同一 token 上下文。

### 构建与检查

- `go test` 覆盖 customerportal 应用层、Postgres 仓储和 HTTP API。
- `npm test --prefix miniapp`
- `npm run typecheck --prefix miniapp`
- `npm run build:mp-weixin --prefix miniapp`

## 手册与需求记录

需要更新：

- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- `orderapp-remote/docs/customer-portal-miniapp-test.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- `orderapp-remote/internal/interfaces/http/support/req_store.go`

手册必须说明：

- 小程序登录使用 ERP 渠道客户账号。
- 运营先在用户权限页把账号设为渠道客户、设置密码并启用登录。
- 再到客户门户配置绑定客户。
- 客户在小程序登录页输入用户名/手机号和密码。
- 需要换账号时进入个人中心点“切换用户”。

## 验收标准

- 小程序登录页没有微信一键登录按钮。
- 渠道客户账号可用 ERP 同用户名/手机号和密码登录小程序。
- 内部员工账号无法登录小程序。
- 未绑定客户门户的渠道账号无法登录小程序。
- 登录成功后进入该账号绑定客户的服务首页或商城首页。
- 首页、商城页、服务页账号操作入口统一为个人中心。
- 个人中心可切换用户和退出登录，返回登录页后能输入另一套账号密码。
- 现有客户能力、主题、商城和订单权限边界不变。
