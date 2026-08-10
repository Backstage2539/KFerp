# PR-591 小程序上拉品牌标识与当前账号防自停用验收记录

## 范围

- 需求：`PR-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD`
- 分支：`codex/mini-pull-brand-elastic-reveal-20260810`
- 目标一：小程序 13 个实际业务页面的品牌尾舱平时为零高度，只在真实内容底部继续上拉手势期间临时展示 `Drived By` 与透明银灰棵凡四字字标；松手后自动回弹隐藏，不能停留空白尾页。
- 目标二：ERP 当前管理员不能关闭自己的登录；后端、前端共同防护，同时保留 BasicAuth 运维恢复通道和其他管理员维护他人的能力。
- 边界：上拉品牌属于只读展示，不写业务数据或操作日志；第二轮纠正只待部署 development，production 代码不操作；服务器部署、小程序固定包、微信开发者工具上传、审核和正式发布是独立检查点。

## DEV 合同

### DEV-591-MINI-PULL-UP-BRAND

- `pages/index/index` 是瞬时启动路由，不挂品牌组件；`pages.json` 其余 13 个实际业务页面统一接入 `PullUpBrandFooter`。
- 13 页使用 `usePullUpBrandGesture` 和普通冒泡的 `touchstart` / `touchmove` / `touchend` / `touchcancel`，不用 prevent/stop，不把全页改造为 `scroll-view`。手势可在同一次拉动中先到达真实底部再继续拉起底标；横向移动或未到底不展开。
- 组件位于正常文档流，不使用 fixed/absolute/sticky。静止时 `max-height: 0` 且不增加滚动区；只在真实底部上拉手势期间临时展开，固定底栏与无底栏页面分别兼顾底栏和安全区。`touchend` / `touchcancel` / `onHide` 立即清除显示状态，`max-height` 约 220ms 收回；松手后自动回弹隐藏，不能停留空白尾页。
- `kefan-wordmark-silver.png` 使用透明背景和银灰单色，去除原图红色、底色和注册标记；“棵凡咖啡”由真实中文字体源码直接生成，避免图片生成模型改写“咖啡”笔画。最终 PNG 为 420×124、9,009 bytes。

### DEV-591-SELF-LOGIN-DISABLE-GUARD

- 当前内部员工提交 `login_enabled=false` 且目标员工等于自身时，`POST /api/auth/account-state` 返回 409 `cannot disable current account`；事务开始前即拒绝，不修改密码状态、不失效会话、不写业务成功审计。
- BasicAuth 管理员没有当前员工 ID，仍可以恢复任意有效内部员工；已登录管理员仍可启停其他员工，启用自身也不受阻止。后端继续校验目标是 active 内部员工并保持原权限要求。
- 员工维护页读取当前操作者并禁用当前员工行的登录开关，提示“当前账号不能关闭自己的登录”；前端仅改善操作引导，绕过页面直接请求仍由后端拒绝。

### DEV-591-DOCS-DEVELOPMENT-DELIVERY

- 同步根目录与 `orderapp-remote/docs` 需求/验收、小程序员工 ERP、客户门户、设置审计手册、PR/DEV 种子、支持合同和本验收记录。
- 完成 mobile auth 定向/API、CompanyStaffView、miniapp 定向/全量、透明资源、类型检查和构建门禁；合入最新 `develop` 后部署 development。
- 不自动把小程序上传、提审或发布到微信，不部署 production 代码；这些状态必须在最终交付中分别说明。

## TDD / 回归证据

- Support RED：`go test ./internal/interfaces/http/support -run TestDev591MiniPullBrandSelfLoginGuardContracts -count=1` 首次在 `req_store.go missing one-line req_product seed PR-591...` 失败。
- Support GREEN：PR/DEV/REV 种子、两套需求验收、三份业务手册、客户履约手册入口、独立验收记录、13 页面接线、透明压缩资源、后端和员工维护防护全部满足合同。
- Auth GREEN：当前员工自停用在数据库事务前返回 409；真实隔离 PostgreSQL 用例确认登录状态和业务审计均未改变，BasicAuth 恢复与管理员维护其他员工回归通过。Vue/Vite 全量为 928/928，构建通过。
- 首轮人工 RED：development 中短页底标提前进入首屏，旧横向字标的“咖啡”笔画失真；新增合同在缺少普通锚点、完整首屏预留和可追溯正确中文字标时 3/3 失败。
- Miniapp GREEN：13 个页面编译 WXML 各且仅一个普通锚点；字标为 420×124、9,009 bytes 的透明 PNG，源码固定为“棵凡咖啡”。32 个测试文件、198 项测试全部通过，类型检查和 development 微信小程序构建通过。
- 第二轮人工 RED：development 中底标继续上拉后可见，但松手后仍停留在大块空白尾页；定向合同在缺少手势 reducer、13 页手势接线和可收回尾舱时失败。
- 第二轮 GREEN：手势 reducer 与页面合同定向 10/10、miniapp 全量 33 个文件 205/205、类型检查和 development 微信小程序构建通过；13 个页面编译 WXML 均为一组普通 `bindtouchstart/move/end/cancel` 且没有 `catchtouchmove`。PR-591 support、后端全包、差异检查和独立代码终审均通过；此处只记录自动门禁，不提前标记为已部署。
- Backend GREEN：`scripts/verify_kferp.sh backend` 全包通过，PR-591 定向 API/支持合同通过；`git diff --check` 与冲突标记检查通过。

## 生产即时恢复

- 已通过现有账号启停 API 恢复本次受影响的目标内部员工登录，不直接修改数据库，也不重置密码、角色或账号类型。
- 恢复请求沿用系统原有权限和审计合同，已确认存在对应账号启用审计；此前确认目标账号有效且密码校验通过，根因是登录状态被关闭。
- 本文和需求种子未记录账号、密码或其他个人信息。即时恢复是生产运维修复，不代表 PR-591 代码已部署 production。

## Van 验收

- [ ] 在无固定底栏页面和有固定底栏页面分别正常浏览，品牌标识平时不可见且没有额外空白滚动区；滚到真实底部后按住继续向上拉，完整出现 `Drived By` 与银灰棵凡四字字标，且不遮挡内容或安全区。
- [ ] 手指保持向上拉动时标识可见；松手、触摸取消或离开页面后约 220ms 内自动回弹隐藏，不能留在标识或空白尾页。
- [ ] 登录页、表单页、长列表和键盘弹起场景不会提前露出品牌；13 个实际业务页面样式一致，瞬时启动页不闪现品牌。
- [ ] ERP 员工维护中，当前管理员的登录开关禁用并显示原因；其他员工登录开关可正常启停。
- [ ] 直接请求关闭当前账号返回 409，当前会话仍有效且操作日志没有业务成功变更；BasicAuth 运维恢复和其他管理员维护他人仍可用。
- [ ] 使用已恢复的 production 目标内部员工账号重新登录；此项只验证即时恢复，不等同于 production 代码验收。

## 交付状态

- `DEV-591-MINI-PULL-UP-BRAND`：done；第二轮松手回弹实现已完成，等待当前自动门禁、合入和 development 部署闭环。
- `DEV-591-SELF-LOGIN-DISABLE-GUARD`：done；production 即时恢复和持久代码防护的 API、数据库状态/审计、Vue 测试与构建均已通过。
- `DEV-591-DOCS-DEVELOPMENT-DELIVERY`：doing；第二轮文档、种子和支持合同已同步，自动门禁、合入、development 部署和新固定开发包证据待补录。
- `REV-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD`：todo，等待 Van 在 development 人工验收。

## Development 部署证据

- 以下为上一版 development 基线证据；第二轮松手回弹纠正版尚未合入或部署。主工作流完成后要用新提交、备份、回滚、smoke 和固定包证据替换当前交付状态；production 代码不操作。

- 纠正功能部署提交：`develop@ca452a5379f0d4c7a197791edef61d4653898c6b`。
- 服务器源码备份：`/opt/stacks/erp/orderapp.backup.deploy-20260810095952-ca452a5379f0`。
- 回滚镜像：`kferp-orderapp-rollback:development-20260810095952-ca452a5379f0`。
- 服务器微信小程序构建产物：`/opt/stacks/erp/orderapp/miniapp/dist/build/mp-weixin`。
- 本机固定开发包：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`；替换前备份：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev.backup-20260810100607-ca452a5379f0`。
- 外部健康检查：HTTP 200。开发包已同步到固定目录，但微信上传、提审和发布未执行；production 代码未部署。
- 服务器与本机固定开发包中的透明字标 SHA-256 均为 `8cb4f61def4cbf8cc96f03aa584283f92ed3380b81138c29ca51e2ff651c91ab`，编译包 13 个实际页面各有一个普通屏幕外锚点。
- 首次纠正部署因服务器根盘被 27.72GB 未使用 BuildKit 缓存写满而在 PostgreSQL 健康检查处失败；仅清理可重建构建缓存后释放约 25GB，数据库自动恢复、旧应用基线恢复 HTTP 200，随后同一提交完整重跑门禁并成功部署。未删除数据库卷、源码备份或业务文件，未修改数据库或业务数据。
