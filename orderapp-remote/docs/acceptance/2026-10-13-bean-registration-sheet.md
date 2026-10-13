# PR-689 价格表底部快捷登记验证

## 范围
底部面板取代登记范围豆单的提示页与整页登录跳转；昵称建议/填写、微信原生手机号确认、所选入口返回、取消及会话复用。后端登记与客户业务权限保持现有规则。

## TDD
2026-10-13 实现前运行小程序行为与 API 测试：共享登记模块缺失、API 错误码未保留，RED。实现后针对所选目标、重复提交、取消、迟到响应、授权失败重试、已有资料恢复和公开/专属权限边界测试 GREEN。

## 自动验证
- 定向小程序：14 项通过，typecheck 与 development 构建通过。
- 独立本机数据库 kferp_bean_sheet_20261013：TestBeanCenterRegistrationPostgres / TestPageEntry.*Postgres 通过。追加登记入口错误码和专属价格不得降级的 HTTP 断言；原有登记、重放、图片、客户隔离、停用、撤回覆盖保留。
- 全量 Go 测试通过；Vue 1,348 项测试及 Vite 构建通过；小程序 46 文件、287 项测试通过，typecheck 与 development 构建通过。
- 日志：/tmp/kferp-bean-sheet-all-tests.log、/tmp/kferp-bean-sheet-mini-tests.log、/tmp/kferp-bean-sheet-typecheck.log、/tmp/kferp-bean-sheet-mini-build.log。
- 开发部署及预览：成功，详见下方交付记录。

## 手册与验收
已更新豆单中心独立手册、页面入口、客户门户说明及 PR/DEV/UT/API/REV。微信原生授权必须由 Van 手机确认；模拟器及自动测试不代表真实手机号验证成功或产品验收。仅交付 DEV 和小程序预览，不发布正式小程序、公众号菜单或生产环境。

## 模拟器交互复查
微信开发者工具 RC 2.02.2607271 / 基础库 3.17.3：首页进入目录、点击咖啡豆标准卡片，底部面板原地显示且页面路径保持豆单中心；空昵称禁用授权、填写并确认昵称后启用授权；“暂不查看”关闭回原目录通过。触摸事件截断影响点击已修正，组件不再使用微信不支持的属性选择器。

模拟器昵称建议曾报告 showNicknameAccessory:fail，手工昵称输入可用；微信昵称建议、手机键盘和原生手机号授权仍需真机确认，不认定为已验收。

## 开发交付
- 功能 PR #195 合入 develop；部署提交 `8418391726728dd65020a37bb6f931bace22d7a5`。部署命令 `KFERP_MINIAPP_DEVELOPMENT_EXPORT_DIR=/Users/yiiiple-work/KFerp-bean-registration-sheet-test-20261013/KFerp-miniapp-mp-weixin-dev ./deploy_orderapp.sh development`。
- 服务器串行 Vue、小程序、Go、Docker 全部检查通过；开发容器运行、数据库 healthy，生产容器仍为原先运行实例。发布日志 `/tmp/kferp-bean-sheet-deploy.log`。
- 回滚目录 `/opt/stacks/erp/orderapp.backup.deploy-20261013014204-841839172672`；镜像 `kferp-orderapp-rollback:development-20261013014204-841839172672`。
- 匿名 `/app/` 303，匿名管理接口 401；认证系统设置、公众号状态、五张需求表接口 200。目录四张表，匿名价格内容均为 `401 mini_registration_required`。日志 `/tmp/kferp-bean-sheet-smoke.log`。
- 已核对新服务号 AppID wxd57133399b1da422、消息加密/凭据就绪、关联小程序 wx6512f8684f584828；菜单发布与网页 OAuth 保持关闭，本次未改动真实菜单。
- 直达入口模拟器复查：登记面板自动出现，关闭后回豆单中心；首次模拟器启动失败，重新编译后通过。最后恢复普通首页启动方式。
- 开发包 `KFerp-bean-registration-sheet-test-20261013/KFerp-miniapp-mp-weixin-dev`，22 页及 88 页面文件验证通过；RELEASE_INFO 指向以上部署提交和开发 API。
- 微信预览上传成功（1,194,633 bytes），二维码 `KFerp-bean-registration-sheet-test-20261013/新版豆单测试二维码.png`，附 `测试说明.txt`。日志 `/tmp/kferp-bean-sheet-preview.log`。以上路径位于 /Users/yiiiple-work/。
- DEV-758/759、UT/API 技术交付完成；PR 保持 review、REV 保持 todo，由 Van 真机确认昵称建议、手机键盘、手机号同意/拒绝、成功返回及复用。

## Van 验收与正式发布授权
2026-10-13，重新登录微信开发者工具后预览上传成功，Van 扫码测试后确认：“可以了，合并到main，并部署到生产环境，给我小程序包，我上传最新小程序”。据此将 PR-689 / REV-689 标记完成，并执行生产发布。此前章节保留当时的待验收状态作为历史记录。

发布分支 `codex/bean-sheet-production-20261013` 合并已验证的 develop `b954a3885d144cceda7dcb0e92aee9c568561fd7` 与最新 main `43ec7232`，保留生产 BOM 兼容修复。正式包必须使用 `https://erp.qacoohee.com/app`，AppID 为 `wx6512f8684f584828`；完成生产配置预检与部署后，随包交付提交、备份和线上检查记录。微信上传、审核和发布由 Van 执行，公众号菜单不在本次自动发布范围。
