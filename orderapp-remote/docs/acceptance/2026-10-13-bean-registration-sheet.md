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
- 开发部署及预览证据：待补。

## 手册与验收
已更新豆单中心独立手册、页面入口、客户门户说明及 PR/DEV/UT/API/REV。微信原生授权必须由 Van 手机确认；模拟器及自动测试不代表真实手机号验证成功或产品验收。仅交付 DEV 和小程序预览，不发布正式小程序、公众号菜单或生产环境。

## 模拟器交互复查
微信开发者工具 RC 2.02.2607271 / 基础库 3.17.3：首页进入目录、点击咖啡豆标准卡片，底部面板原地显示且页面路径保持豆单中心；空昵称禁用授权、填写并确认昵称后启用授权；“暂不查看”关闭回原目录通过。触摸事件截断影响点击已修正，组件不再使用微信不支持的属性选择器。

模拟器昵称建议曾报告 showNicknameAccessory:fail，手工昵称输入可用；微信昵称建议、手机键盘和原生手机号授权仍需真机确认，不认定为已验收。
