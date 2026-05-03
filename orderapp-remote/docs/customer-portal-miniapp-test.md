# 客户服务小程序联调手册

## 目标

在正式小程序发布前，用测试小程序或微信开发者工具打通：

- 微信小程序登录
- ERP 客户门户 session
- 小程序用户绑定 ERP 客户
- 客户首页按能力包显示入口

## 后端环境变量

真实微信测试号模式：

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

两种模式不要同时用于正式验收。真实微信模式优先；没有 AppSecret 或只想快速验证客户绑定时，使用稳定模拟登录模式。

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

## 客户绑定 SQL

登录一次小程序后，查最近的小程序用户：

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

## 验收

1. 打开微信开发者工具。
2. 导入 `miniapp/dist/build/mp-weixin`。
3. 点击“微信一键登录”。
4. 如果是第一次登录，先按上面的 SQL 绑定客户。
5. 再次进入小程序首页。

通过标准：

- 首页显示绑定客户名称。
- 首页只显示已开通的能力入口。
- 未携带 token 请求 `/app/api/mini/me` 返回 `401 {"error":"mini token required"}`。
- 生产环境未配置微信参数且未打开 dev login 时，`/app/api/mini/login` 返回 `503 {"error":"mini login disabled"}`。
