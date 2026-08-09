# PR-591 小程序上拉品牌标识与当前账号防自停用验收记录

## 范围

- 需求：`PR-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD`
- 分支：`codex/mini-pull-brand-self-login-guard-20260810`
- 目标一：小程序 13 个实际业务页面在正常浏览时不显示品牌标识，到达内容底部后继续向上拉才展示 `Drived By` 与透明银灰棵凡四字字标。
- 目标二：ERP 当前管理员不能关闭自己的登录；后端、前端共同防护，同时保留 BasicAuth 运维恢复通道和其他管理员维护他人的能力。
- 边界：上拉品牌属于只读展示，不写业务数据或操作日志；production 仅完成目标账号即时恢复，不部署本需求代码；服务器部署、小程序固定包、微信开发者工具上传、审核和正式发布是独立检查点。

## DEV 合同

### DEV-591-MINI-PULL-UP-BRAND

- `pages/index/index` 是瞬时启动路由，不挂品牌组件；`pages.json` 其余 13 个实际业务页面统一接入 `PullUpBrandFooter`。
- 组件位于正常文档流，不使用 fixed。正常浏览和刚到内容底部不显示；继续向上拉时，保留区之后展示 `Drived By` 和横向棵凡四字字标。固定底栏页面增加对应底栏预留，无底栏页面使用较小预留，并兼顾 `safe-area-inset-bottom`。
- `kefan-wordmark-silver.png` 使用透明背景和银灰单色，去除原图红色、底色和注册标记；资源压缩并保持合理的横向宽高，不明显增加微信包体。

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
- Miniapp GREEN：字标为 420×124、10,377 bytes 的透明 PNG；32 个测试文件、198 项测试全部通过，类型检查和 development 微信小程序构建通过。
- Backend GREEN：`scripts/verify_kferp.sh backend` 全包通过，PR-591 定向 API/支持合同通过；`git diff --check` 与冲突标记检查通过。

## 生产即时恢复

- 已通过现有账号启停 API 恢复本次受影响的目标内部员工登录，不直接修改数据库，也不重置密码、角色或账号类型。
- 恢复请求沿用系统原有权限和审计合同，已确认存在对应账号启用审计；此前确认目标账号有效且密码校验通过，根因是登录状态被关闭。
- 本文和需求种子未记录账号、密码或其他个人信息。即时恢复是生产运维修复，不代表 PR-591 代码已部署 production。

## Van 验收

- [ ] 在无固定底栏页面和有固定底栏页面分别正常浏览，品牌标识平时不可见；滚到底后继续向上拉，完整出现 `Drived By` 与银灰棵凡四字字标，且不遮挡内容或安全区。
- [ ] 登录页、表单页、长列表和键盘弹起场景不会提前露出品牌；13 个实际业务页面样式一致，瞬时启动页不闪现品牌。
- [ ] ERP 员工维护中，当前管理员的登录开关禁用并显示原因；其他员工登录开关可正常启停。
- [ ] 直接请求关闭当前账号返回 409，当前会话仍有效且操作日志没有业务成功变更；BasicAuth 运维恢复和其他管理员维护他人仍可用。
- [ ] 使用已恢复的 production 目标内部员工账号重新登录；此项只验证即时恢复，不等同于 production 代码验收。

## 交付状态

- `DEV-591-MINI-PULL-UP-BRAND`：done；自动测试、类型检查和 development 构建通过，等待 development 部署及 Van 手工检查上拉手感。
- `DEV-591-SELF-LOGIN-DISABLE-GUARD`：done；production 即时恢复和持久代码防护的 API、数据库状态/审计、Vue 测试与构建均已通过。
- `DEV-591-DOCS-DEVELOPMENT-DELIVERY`：doing；文档与种子已登记，测试、合并和 development 部署待补证据。
- `REV-591-MINI-PULL-BRAND-SELF-LOGIN-GUARD`：todo，等待 Van 在 development 人工验收。
