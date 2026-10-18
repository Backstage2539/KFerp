# PR-702 小程序登录入口与主动退出

## 范围与原因

- 首页账号登录入口以及员工、客户、仅登记访客的个人中心退出流程。
- 开发包在微信开发者工具实测复现：退出后闪回原个人中心；首页登录进入空白启动页，控制台出现 `navigateTo:fail timeout`。
- 原因：显式退出与普通会话过期都触发默认微信身份恢复，异步恢复完成后再次跳转，覆盖用户主动登录/退出意图。
- 修复：显式入口打开账号密码表单；当前环境持久保存主动退出状态；主动再次登录才解除；恢复中的迟到响应不能重建退出会话；离开登录页或切换密码模式后不再自动跳转。普通令牌过期恢复保持原有规则。
- 无业务数据或权限规则变更，无数据库迁移；现有登录 API 不变。

## RED / GREEN

- RED：新增密码路由测试在原实现中缺少 `mode=password`；主动退出测试在原实现中仍调用微信恢复；初始空令牌的迟到登录响应仍写回会话。原 store 不存在 `logout()`，持久退出测试失败。
- 本机证据：`/tmp/kferp-login-red.log`、`/tmp/kferp-login-store-red.log`。初次 RED 中另有缺少 API 环境配置的测试设置错误，已修正，不计作业务复现。
- GREEN：`npm test -- src/stores/session.test.ts src/utils/loginReturn.test.ts src/utils/registrationSession.test.ts`，16 项通过。
- 全部小程序测试：338 项 / 56 文件通过；`npm run typecheck` 和 `npm run build:mp-weixin:development` 通过；24 个声明页面文件完整。
- API 客户端回归：现有恢复测试覆盖有效 token、401 后重登、403/网络错误、更新 token；新测试覆盖主动退出不请求以及迟到响应丢弃。服务端登录 API 未修改。
- 合并前全量检查：Go 全量通过；Vue 1,404 项及构建通过；小程序 338 项、类型检查及开发构建再次通过。订单复制的 sales/customerportal/support PostgreSQL 定向测试通过，使用独立测试 schema，不改业务数据；未将未配置数据库的全量 Go 结果解释为全库集成验收。

## DevTools 交互证据

- 当前项目为 development 包，API 域名 `dev.qacoohee.com`。
- 首页“登录，查看我的业务”稳定打开“员工 / 客户账号”表单。
- 使用临时测试文字确认用户名与密码输入框均可输入；随后清空，未提交真实账号密码。
- 返回个人中心点击“退出登录”，稳定停留在账号密码表单，不再闪回个人中心。
- 冷启动退出标记由重建 Pinia store 的持久化测试覆盖；未将后续人工登录状态作为自动化冷启动验收证据。
- 不保存或传播个人中心的手机号、用户名称等隐私截图。

## 手册与交付

- 更新 `OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`、`OP_MANUAL_CUSTOMER_PORTAL.md`，现有 Vue 手册入口读取同一文档；登录页面同步账号使用提示。
- PR-702 / DEV-793 写入需求进度来源；业务验收保持待确认。
- Van 于 2026-10-19 要求合入 develop、main 并部署生产。生产发布将包含 develop 已验证的订单详情文字复制等累计更新；微信上传、审核、发布仍独立处理。
- 合并版本、部署备份、运行检查和正式小程序包将在交付后追加。

## 生产集成检查

- 发布分支从 `origin/main@34a311d1` 合入已验证的 `origin/develop@ab573007`；保留 main 专有的 BOM 物料产出历史回填保护。进度文档冲突保留两边交付记录。
- 新增聚焦数据库测试 `TestProductionMaterialBackfillReleasePostgres`：移除生产已有保护时，在重复 schema 初始化时触发产出绑定约束错误（RED）；恢复原 main 保护后通过（GREEN）。此处不改变业务代码。
- 聚焦迁移测试及四端订单复制数据库测试全部通过；生产合并后的 Go 全量通过，网页和小程序源码与已验证 develop 一致。
- 补跑的历史大测试 `TestPR598MaterialOutputRepositoryAndCompatibilityMigrationPostgres` 在旧样例将产出物料标为外购时失败；试验修正后又遇到旧规格模式字段缺失。已撤回试验修改、保留原测试，新增独立迁移回归避免依赖这组过期全流程样例。该大测试不计入通过项；未声称全库数据库集成测试通过。
- 生产数据库发布前备份：`/opt/stacks/erp-production/backups/before-login-order-copy-20261019.dump`，12 MiB；`pg_restore --list` 校验通过。SHA-256：`bcd443a5e301c86f1aefa504b1325b51877fe2017ab359860db229f1cccd4e2a`。备份仅在生产服务器保存。

## 开发与生产交付回执

- 修复分支 `codex/fix-miniapp-login-entry-20261019` 已推送，提交 `a061480e`；开发集成、部署版本 `ab573007a33c5ee373b7dd5402f0e7ba2795d9fd`。
- 生产发布分支 `codex/release-login-order-copy-20261019` 已推送，提交 `0e4cef9a`；合入 main 后生产部署版本 `c815fb9905fb8d0a80912ec8933b0788de23dbdc`。
- 两个环境分别执行 `./deploy_orderapp.sh development` / `./deploy_orderapp.sh production`，指定本次独立交付目录；服务器 Vue 1,404 项、小程序 338 项、类型检查、构建、Go 全量及镜像内 Go 测试全部通过。两次发布脚本均完成，外部登录页 HTTP 200。
- 开发源码备份 `/opt/stacks/erp/orderapp.backup.deploy-20261019002107-ab573007a33c`；回滚镜像 `kferp-orderapp-rollback:development-20261019002107-ab573007a33c`。
- 生产源码备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261019003001-c815fb9905fb`；回滚镜像 `kferp-orderapp-rollback:production-20261019003001-c815fb9905fb`。
- 运行镜像：development `sha256:7f460bc19f0f38bd1b50d1a34bb894ddb2ed626bf63120f11c969f02d02d752e`；production `sha256:6f32c31c18ef8433ad39f20aea01fcf3bd59dc620a088855219d8bab470f8261`。两边应用 running、数据库 healthy。
- 两边只读检查通过：登录页及 Vue 订单入口 200；无效 ID 的 ERP 复制接口认证后 400；四个复制入口匿名均 401；员工手册含主动退出说明；PR-701/702 为 review，DEV-793 为 done。未读取真实订单做生产业务验收。
- 生产首次认证冒烟误用 env 文件中的凭据，收到 401；改用容器实际运行凭据后通过，凭据未输出，未修改认证配置。检查脚本及前后日志保留在交付目录 evidence 中。
- 开发包：`/Users/yiiiple-work/KFerp-login-fix-delivery-20261019/KFerp-miniapp-dev-ab573007.zip`；SHA-256 `dcc680cbcd9def148b8e2d9387574ad8d5b88b95c97415d9ccc2621fb9bdb10f`。
- 生产包：`/Users/yiiiple-work/KFerp-login-fix-delivery-20261019/KFerp-miniapp-production-c815fb99.zip`；SHA-256 `b8b70959655ff6cba6bf7924550549ffbcba7712276005dd98c79760490cb27b`。
- 两包均含 RELEASE_INFO、PAGE_FILE_MANIFEST，24 页面/96 个页面文件完整，ZIP CRC 通过；生产包仅含生产 API 地址。生产源目录为同交付目录下 `KFerp-miniapp-mp-weixin`，开发源目录为 `KFerp-miniapp-mp-weixin-dev`。
- 未替换用户当前正在使用的 DevTools 项目，早前测试目录保留修复预览和旧包备份；正式后续预览/上传使用本次交付目录。微信上传、审核、发布及 Van 业务验收未执行。
- 回执随后合入 develop/main，属于文档更新；运行代码和小程序包版本仍为上述提交，不为回执再次重启服务。
