# 客户服务小程序联调手册

## 目标

在正式小程序发布前，用测试小程序或微信开发者工具打通：

- ERP 渠道客户账号密码登录
- ERP 客户门户 session
- ERP 渠道客户账号绑定 ERP 客户
- 客户首页按能力包显示入口

## 后端环境变量

后端仍可保留真实微信测试号配置用于历史联调，但当前用户入口不再展示微信一键登录。小程序正式验收使用 ERP 账号密码登录：

```text
POST /app/api/mini/login/password
{
  "login": "用户名或手机号",
  "password": "密码"
}
```

真实微信测试号模式仅作兼容说明：

```bash
WECHAT_MINI_APP_ID=<测试小程序 AppID>
WECHAT_MINI_APP_SECRET=<测试小程序 AppSecret>
CUSTOMER_PORTAL_DEV_LOGIN=false
```

稳定模拟登录模式：

```bash
CUSTOMER_PORTAL_DEV_LOGIN=true
CUSTOMER_PORTAL_DEV_OPENID=dev-openid-van-test
CUSTOMER_PORTAL_DEV_UNIONID=
```

正式验收不要依赖 openid 登录或稳定模拟 openid；客户应在登录页输入 ERP 渠道客户账号的用户名/手机号和密码。

## 小程序构建

默认接口地址是线上开发栈：

```bash
VITE_KFERP_API_BASE=https://erp.qacoohee.com/app npm run build:mp-weixin --prefix miniapp
```

微信开发者工具导入：

```text
miniapp/dist/build/mp-weixin
```

开发者工具里填测试小程序 AppID。联调阶段可在“详情 -> 本地设置”勾选“不校验合法域名、web-view、TLS 版本以及 HTTPS 证书”。如果不勾选，需要在小程序后台把 `https://erp.qacoohee.com` 加入 request 合法域名。

## 客户账号准备

1. 在 ERP 客户履约运营台选择目标客户。
2. 在“外部用户”区域创建客户账号，填写姓名、手机号和初始密码。
3. 确认外部用户登录已启用。
4. 客户门户配置页仅用于确认客户能力模板和跳转履约运营台，不再直接创建或绑定账号。
5. 小程序登录页输入用户名/手机号和密码。
6. 需要换另一个账号时，进入个人中心，点击“切换用户”，回登录页重新输入。

## 旧 openid 客户绑定 SQL

以下 SQL 只用于排查历史 openid 登录或旧测试数据；新登录链路会由 `/api/mini/login/password` 根据 ERP 渠道客户账号自动同步小程序用户绑定。

登录一次旧 openid 小程序后，查最近的小程序用户：

```sql
SELECT id, openid, nickname, phone, last_login_at
FROM p2rms15pepb5ciz.mini_users
ORDER BY id DESC
LIMIT 10;
```

查要绑定的 ERP 客户：

```sql
SELECT id, name, company_name, phone
FROM p2rms15pepb5ciz.customers
WHERE name ILIKE '%客户名%'
ORDER BY id DESC
LIMIT 20;
```

绑定小程序用户和客户，并开通能力包：

```sql
\set mini_user_id 1
\set customer_id 1

INSERT INTO p2rms15pepb5ciz.customer_portal_profiles(customer_id, display_name, status, enabled, updated_by)
VALUES(:customer_id, '', 'active', true, 'miniapp-test')
ON CONFLICT(customer_id) DO UPDATE SET
  enabled=true,
  status='active',
  updated_at=now(),
  updated_by='miniapp-test';

INSERT INTO p2rms15pepb5ciz.customer_portal_user_bindings(mini_user_id, customer_id, role, status, approved_by)
VALUES(:mini_user_id, :customer_id, 'owner', 'approved', 'miniapp-test')
ON CONFLICT(mini_user_id, customer_id) DO UPDATE SET
  role='owner',
  status='approved',
  approved_by='miniapp-test';

INSERT INTO p2rms15pepb5ciz.customer_service_capabilities(customer_id, capability_code, enabled, config_json)
SELECT :customer_id, capability_code, true, '{}'::jsonb
FROM unnest(ARRAY[
  'bean_list',
  'product_order',
  'direct_ship',
  'processing',
  'inventory_custody',
  'shipping_query',
  'settlement'
]) AS capability_code
ON CONFLICT(customer_id, capability_code) DO UPDATE SET
  enabled=true,
  config_json=EXCLUDED.config_json,
  updated_at=now();
```

## 小程序主题配置

ERP 后台路径：`设置 -> 客户门户配置`。

1. 搜索要验收的小程序绑定客户。
2. 在客户行内确认“门户启用”。
3. 在“小程序主题”中选择一套主题：
   - 咖啡工厂专业风：默认主题，暖咖啡色，适合大多数客户。
   - 清爽业务工具风：克制清楚，适合高频查订单、物流和库存。
   - 品牌会员高级风：质感更强，适合合作伙伴入口。
4. 点击“保存配置”。
5. 重新进入微信开发者工具中的小程序，或切换客户后回到首页，确认首页和服务页跟随新主题。

未配置主题的历史客户默认使用“咖啡工厂专业风”。小程序用户端不提供自行切换主题入口。

## 验收

1. 打开微信开发者工具。
2. 导入 `miniapp/dist/build/mp-weixin`。
3. 在登录页输入 ERP 渠道客户账号的用户名/手机号和密码。
4. 登录成功后进入小程序首页、服务首页或商城首页。
5. 进入个人中心，确认可看到当前客户；点击“切换用户”应回到登录页。

通过标准：

- 登录页只展示用户名/手机号、密码和登录按钮，不展示微信一键登录。
- 首页显示绑定客户名称。
- 首页只显示已开通的能力入口。
- 首页、商城页和服务页顶部通过“个人中心”进入账号操作，不直接堆放退出登录。
- 个人中心可切换用户和退出登录。
- 首页和服务页视觉主题与 ERP 客户门户配置中选择的主题一致。
- 未配置主题时，小程序默认使用咖啡工厂专业风。
- 未携带 token 请求 `/app/api/mini/login/password` 不需要 token；账号密码错误返回非 500 JSON 错误。
- 内部员工账号和未绑定客户门户的渠道账号不能登录小程序。
- 未携带 token 请求 `/app/api/mini/me` 返回 `401 {"error":"mini token required"}`。
- 生产环境未配置微信参数且未打开 dev login 时，`/app/api/mini/login` 返回 `503 {"error":"mini login disabled"}`。
