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
