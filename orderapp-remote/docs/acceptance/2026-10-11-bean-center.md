# PR-688 小程序豆单中心与简化登记：验证记录

## 范围与交付边界

功能分支 `codex/miniapp-bean-center-20261011`，基线 `origin/develop@31e9bbcaf720bde5c2edcc733cf7db30d75d631c`。交付开发环境及小程序开发测试包；正式微信菜单与小程序发布由后续上线完成，Van 产品验收尚未完成。

- 独立登记资料只保存服务器验证的手机号、昵称及时间，不创建 ERP 客户。唯一有效账号匹配复用既有客户绑定，普通访客无订单/专属价格权限。
- 手工选择的已发布价格入口构成目录；登记权限、图片、停用、撤回和网页指引使用同一权限检查。现有价格表不会自动生成或替换卡片。
- 新目录、首页/底部导航、登记/资料修改、登录返回、多客户选择；员工业务导航保留。
- 旧公众号最近订单回调只返回小程序指引；绑定记录保留。菜单编辑可生成三个导航草稿，不自动发布。
- 登记/验证手机号、昵称修改、账号停用写入操作日志，日志不复制完整手机号或微信凭证。

## TDD 证据

实现前确认 RED：登记登录模式无效；新登记 HTTP 路由 405；registered 范围校验失败；豆单辅助模块缺失；Vue 默认草稿缺目录字段及菜单路径错误。

发布复查补充 RED：登记及资料修改操作日志数量为 0；过期恢复未保留登记提示。修复后 GREEN。

## 自动验证

| 层次 | 验证 | 结果 |
| --- | --- | --- |
| Go / ERP | `bash scripts/verify_kferp.sh all` | 全 Go、1,348 项 Vue 测试、Vue/Vite 构建通过 |
| PostgreSQL / HTTP | `ORDERAPP_TEST_DATABASE_URL=… go test ./internal/appmain -run 'TestBeanCenterRegistrationPostgres|TestPageEntry.*Postgres|TestWechatOfficial' -count=1` | 本机独立临时 schema 通过，非线上数据库 |
| 小程序 | `npm test` | 45 文件、276 项测试通过 |
| 小程序 | `npm run typecheck`、`npm run build:mp-weixin:development` | 通过；目标 `https://dev.qacoohee.com/app` |

数据库/接口测试覆盖：无客户新登记、微信凭证无效/重放/手工号码拒绝、昵称维护、旧手机号不污染验证资料、操作日志、初始零卡片/手工四张/排序、新发布不加卡片、草稿隔离和手工发布、专属目标拒绝、停用/撤回、唯一账号匹配、多客户选择、跨客户拒绝、订单分页参数、重号不授权、图片接口权限、管理查询分页/匿名拒绝/停用、会话过期恢复、停用后不能重新登记。

测试中保留 ERP 单账号单客户约束；多客户使用小程序已有人工批准关联。重号测试只在临时 schema 移除电话唯一索引来模拟历史歧义数据，生产和开发数据库索引不变。微信身份与手机凭证由测试替身提供，不能据此认定真机授权已通过。

本地输出：`/tmp/kferp-bean-all-tests.log`、`/tmp/kferp-bean-db-tests.log`、`/tmp/kferp-bean-mini-tests.log`、`/tmp/kferp-bean-mini-build.log`。

## 手册与入口

新增 `docs/OP_MANUAL_BEAN_LIST_CENTER.md`，ERP 页面入口管理及客户门户登记列表可打开。同步公众号、页面入口、客户门户、系统设置手册及手册总索引。PR/DEV/UT/API/REV 保留相互独立状态，REV 由 Van 验收。

## 开发环境配置与待验收

部署后已将已有四个测试入口设置为登记用户可见、目录展示和排序，不改变其他入口或历史权限：咖啡豆标准、咖啡豆一件代发、咖啡生豆、挂耳。目标发布版本仍由管理员手工切换。

- PR #192 已合入 develop，首轮部署提交 `ce911c5bdbbe36e8d3edf92e60d666036f6c94fe`。开发容器运行，部署健康检查及外部 `/app/login` 返回 200。部署日志 `/tmp/kferp-bean-deploy.log`。
- 回滚备份 `/opt/stacks/erp/orderapp.backup.deploy-20261011020835-ce911c5bdbbe`，镜像 `kferp-orderapp-rollback:development-20261011020835-ce911c5bdbbe`。未部署生产环境。
- 目录 API 恰好返回四张表，排序 10/20/30/40；匿名直接访问四张价格内容均为 401。原有其他草稿、46 个历史入口未改动。日志 `/tmp/kferp-bean-live-smoke.log`。
- 微信开发者工具导入开发包成功；22 个页面及 88 个页面文件完整。测试包 `/Users/yiiiple-work/KFerp-bean-center-test-20261011/KFerp-miniapp-mp-weixin-dev`。
- 模拟器观察：首页进入豆单、四卡片及版本、访客三项导航、打开受限价格转登记提示、手机号/昵称登记页及返回目录通过。未填写昵称时授权按钮禁用。
- ERP 390px 窄屏卡片/标签及编辑字段可用；帮助返回恢复页面入口 Tab、原表编辑、登记范围及排序。网页受限入口只显示小程序指引，未出现密码登录或公开价格。
- 截图保存在测试包上级目录：`bean-center-simulator.png`、`registration-simulator.png`、`admin-390px.png`。
- 微信开发者工具 CLI 预览上传两次返回 `41002 appid missing`，本地项目 AppID 已正确配置为 `wx6512f8684f584828`。尚无可交付预览二维码，不能认定真机授权通过。错误记录 `/tmp/kferp-bean-preview.log`、`/tmp/kferp-bean-preview-retry.log`。
- 交付复查同步清理客户门户/页面入口/公众号手册和状态提示中的旧绑定码、四导航、必须已有 ERP 客户等过时表述；本次跟进不改变小程序功能。

真实手机号授权、拒绝授权、再次进入、手机号修改及业务内容由 Van 真机验收。公众号正式菜单待正式小程序发布后切换；当前未发布真实菜单或小程序。

## 交付后说明复核

- PR #193 已合入并部署 `0ae6bb2be8f1fa5d99b36aaaf36a5ca73d6fa873`。外部登录 200、容器 running/0 restarts、四卡片及匿名 401 复验通过；同提交 22 页/88 文件开发包已导出。日志 `/tmp/kferp-bean-delivery-deploy.log`、`/tmp/kferp-bean-final-smoke.log`。
- 开发者工具关闭重开后，预览上传仍返回 41002。独立测试说明及打包文件已生成；真实手机号和 Van 验收仍待完成。
- 审查指出两处手册歧义，按实际密码登录与网页授权代码修正：分别列出内部员工/渠道客户条件；保留当前网页自动识别使用 UnionID 的配置步骤，并明确新小程序登记不依赖网页配置。公众号管理状态文案同步纠正。
