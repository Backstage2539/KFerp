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

不要在 Mac 上执行 `npm ci`、类型检查或 UniApp 构建。统一走仓库根目录的远程发布脚本：

```bash
# 已推送功能分支：只在服务器做隔离预检，不部署、不重启
./deploy_orderapp.sh --preflight development

# develop 分支：服务器串行测试并构建开发 API 包
./deploy_orderapp.sh development

# main 分支：服务器串行测试并构建正式 API 包，随后拉回固定目录
./deploy_orderapp.sh production
```

功能分支必须先通过 `--preflight` 才能合入 `develop`。预检使用唯一临时镜像标签，结束后清理临时源码、依赖、构建目录和镜像；不会改写 `/opt/stacks/erp*`、Compose 文件、运行中容器或 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`。

API 地址由脚本强制按环境写入：development 为
`https://dev.erp.qacoohee.com/app`，production 为
`https://erp.qacoohee.com/app`。生产构建成功后，微信开发者工具只导入固定目录：

```text
/Users/yiiiple-work/KFerp-miniapp-mp-weixin
```

不要再导入功能分支、临时 worktree 或 `miniapp/dist/build/mp-weixin` 旧目录。导入固定目录后先清理构建缓存并重新编译，再核对界面字段和请求域名。

导入前打开固定目录中的 `RELEASE_INFO`，确认 `environment=production`、`api_base=https://erp.qacoohee.com/app`，并且 `commit` 等于本次发布的 `origin/main`。同级带 `backup-时间-commit` 后缀的目录是上一份本机产物，可用于独立回滚预览；不要把备份目录当作本次新版本上传。

**重要：服务器 ERP 部署与微信小程序发布不是同一步。** 远程脚本完成，只代表后端已部署且小程序代码包已生成/同步；仍需在微信开发者工具点击“上传”，到微信公众平台提交审核并发布，用户微信里的版本才会更新。

开发者工具里填测试小程序 AppID。联调阶段可在“详情 -> 本地设置”勾选“不校验合法域名、web-view、TLS 版本以及 HTTPS 证书”。如果不勾选，需要在小程序后台把 `https://erp.qacoohee.com` 同时加入 request 合法域名和 downloadFile 合法域名；商品价格表 `PDF`、`长图` 都走 `downloadFile`。

## 客户账号准备

1. 在 ERP 客户门户配置中找到目标客户。
2. 在该客户行的“外部用户”区域创建客户账号，填写姓名、手机号和初始密码。
3. 确认外部用户登录已启用。
4. 外部用户配置了手机号、密码并启用登录后，该客户会出现在客户履约运营台选择器中。
5. 小程序登录页输入用户名/手机号和密码。
6. 需要换另一个账号时，进入个人中心，点击“切换用户”，回登录页重新输入。
7. 小程序客户侧提单只按实价提交，不显示运费和优惠输入；如需做优惠或运费调整，转到 ERP 内部录单或改单处理。

## 我的商品联调

前置数据：

1. ERP 产品价格表已为该客户或公共兜底发布可见的工厂供货商品价格表。
2. ERP 阶梯价模板已打开“允许客户转售豆单使用”，且模板展示单位能匹配来源豆单价格单位。
3. 客户门户能力模板启用 `bean_list`，当前小程序账号绑定该客户。

接口检查：

```text
GET /app/api/mini/resale-bean-lists
GET /app/api/mini/customer-products
POST /app/api/mini/customer-products/categories
PUT /app/api/mini/customer-products/categories/:id
DELETE /app/api/mini/customer-products/categories/:id
POST /app/api/mini/customer-products/categories/:id/move
POST /app/api/mini/customer-products/:id/category
GET /app/api/mini/resale-bean-lists/:source_id/editor
POST /app/api/mini/resale-bean-lists/drafts
POST /app/api/mini/resale-bean-lists/publications
GET /app/api/mini/resale-bean-lists/:id.pdf
GET /app/api/mini/resale-bean-lists/:id.png
```

开发者工具操作：

1. 登录小程序后确认首页不显示“我的商品”，进入 个人中心 → 我的商品。
2. 确认页面展示“商品分类”“商品价格表”“已发布商品价格表”和“我的价格表设置”。
3. 在“商品分类”新增一级/二级分类，改名、上移/下移、删除，并把客户商品归类；第一次编辑公共分类时，ERP 应派生客户专属分类模板。
4. 确认“商品价格表”按 `list_type/list_type_label` 分组显示商品数量、价格表数量和当前工厂供货版本。
5. 确认“已发布商品价格表”按类型折叠，默认只显示最新版本摘要。
6. 在“我的价格表设置”选择来源工厂供货商品价格表、授权阶梯价模板，修改版本号、品牌名、价格表说明、版本说明、预设背景色/文字色、样式、商品勾选、统一加价、倍率加价和上新/推荐标签；页面不能出现覆盖档位、单品价或裸色值输入。
7. 点击“保存草稿”，确认接口返回当前客户草稿，ERP 操作日志可查。
8. 点击“发布商品价格表”，确认“已发布商品价格表”出现新版本，来源工厂供货商品价格表未被改写。
9. 点击“PDF”和“长图”，确认 PDF 可打开并显示菜单、PNG 长图可预览；背景、logo、标签、价格、版本号和分页/长图布局不重叠。
10. 用另一个客户账号登录，确认不能读取或下载上一个客户的商品价格表。

失败排查：

- 如果列表没有授权模板：检查 ERP 阶梯价模板是否 active 且已打开“允许客户转售豆单使用”。
- 如果发布提示价格不匹配：检查来源供货豆单是否有对应档位价格，以及模板展示单位是否与来源价格单位一致。
- 如果 PDF/长图打不开：先看开发者工具 Console 是否出现 `downloadFile 合法域名校验出错`。出现该错误时，在微信后台把 `https://erp.qacoohee.com` 加入 downloadFile 合法域名，或在开发者工具“详情 -> 本地设置”关闭合法域名校验后重新编译。域名无误后，再检查 mini token、当前客户绑定、`bean_list` 能力和 `bean_list_publication_assets` 缓存记录。

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
2. 导入 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`，清理缓存后重新编译。
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

## 工厂商品表联调（PR-434-MINIAPP-FACTORY-PRODUCT-TABLES-SPLIT）

1. 从对应发布分支运行远程部署脚本；不要在 Mac 本地安装依赖或构建。
2. 生产包由脚本同步后，微信开发者工具导入路径为 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`。
3. 使用已绑定 `bean_list` 能力的客户账号登录，进入底部 `个人中心`。
4. 点击 `工厂商品表`，确认页面按商品类型展示当前客户可见的最新工厂商品价格表；大类和快照分类都可以收起展开。
5. 点击价格表右侧 `PDF`，应打开文档预览并显示系统菜单；点击 `长图`，应进入图片预览。

## 我的商品联调（PR-434-MINIAPP-FACTORY-PRODUCT-TABLES-SPLIT）

1. 在 `个人中心` 点击 `我的商品`，首屏只应看到客户商品/价格表摘要、`已发布商品价格表` 折叠版本列表和 `价格表设置` 按钮。
2. 点击 `价格表设置`，复制来源可选 `工厂价格表` 或 `我的已发布价格表`。
3. 从旧的客户已发布价格表复制时，检查分类、上新/推荐标签、标红词、品牌说明和样式被带入；发布时价格仍按原工厂价格来源计算，不重复加价。
4. 在设置页新增、删除、改名分类；展开分类后勾选商品，点击商品右侧 `商品配置` 设置 `上新`、`推荐` 和标红词。
5. 发布后自动回到 `我的商品`，展开对应类型版本列表，点击 `PDF` 和 `长图` 能直接打开输出。
