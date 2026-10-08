# PR-687 — 统一页面入口管理

## 范围与状态

独立分支 `codex/unified-page-entries-20261009`，基线 `origin/develop@df9c83ba`。页面入口手工创建，初始零条，不自动创建当前四张表。历史入口保持原路径、目标、状态和权限，只查看、停用。公众号消息、订单、小程序发布不自动启用。产品验收由 Van 完成。

PR-687；DEV-749（数据）、DEV-750（管理界面）、DEV-751（两端）、DEV-752（网页认证）、DEV-753（手册及开发交付）；UT-687 / API-687 / REV-687。

## RED → GREEN

- 实现前新增 `application/pageentry/model_test.go`、`appmain/page_entry_integration_test.go` 和 Vue `page-entries.test.js`。Go 因新包/模型缺失失败；Vue 因 page-entries 模块缺失失败。
- 实现后在独立 PostgreSQL schema 运行真实 API/数据库测试，保留原公众号绑定、查询及回调回归。
- 旧“类型 × 两个用途自动补齐”测试改为停止生成及历史记录逐字段保留测试，原价格版本隔离、专属权限、撤回、绑定、订单和加密回调测试保留。

## 自动验证

- `bash scripts/verify_kferp.sh all`：Go 全量测试、架构边界检查、Vue 1,346 项测试、Vue/Vite 双页面构建通过。
- 隔离测试 PostgreSQL 上，`go test -race ./internal/appmain -run 'TestPageEntry|TestWechatOfficial' -count=1` 通过。测试 schema 自动清理，没有使用线上业务数据。
- 小程序 `npm run typecheck`、268 项测试通过；以 development + `https://dev.qacoohee.com/app` 构建 `mp-weixin` 通过。没有上传微信。
- `git diff --check` 通过。

覆盖：初始零条、手工四张实际价格表、复制表/新版本不增入口、名称/目标变化路径稳定、公开及客户专属价格表、草稿发布隔离、乐观版本冲突、停用/软删除/撤回、旧路径状态不变、菜单引用。

图片覆盖发布成员检查、草稿不可见、其他入口图片拒绝、停用及解绑后拒绝。认证覆盖 OpenID 有效绑定、可信 UnionID 自动匹配、密码兜底不建立公众号绑定、微信多客户先选择（现有 ERP 密码账号受单客户有效绑定约束）、网页选择不改公众号查询客户、跨客户拒绝、解绑/账号停用/密码登录停用实时失效、安全 Cookie、跨站登录拒绝、状态过期、不同浏览器拒绝、重复授权回调拒绝、失败回到原本站页面、错误尝试限制。

## 界面技术检查

使用本地 Vite、无线上账号的固定 API 测试数据，分别检查 1360 px 与 390 px：系统设置四个 Tab、入口编辑、手机预览、手册返回编辑项、独立网页图文展示。无横向溢出、无运行时错误。窄屏入口列表显示为逐项卡片，编辑区单列。

本地检查脚本 `/private/tmp/kferp-page-layout-check.mjs`；截图 `/private/tmp/kferp-pages-admin-1360.png`、`/private/tmp/kferp-pages-admin-390.png`、`/private/tmp/kferp-pages-web-390.png`。这些是界面技术检查，不是线上业务流程验收。

## 复查与手册

新增独立《页面入口管理》，并更新公众号、系统设置、客户门户、价格表、手册索引和导航。发布数据与草稿分开；图片只经受控接口读取；客户网页不会读取 ERP 本地令牌。返回地址只由服务端生成的入口标识构造。微信配置独立于消息开关，默认关闭；服务器只下发状态。

旧自动生成触发器移除；删除会保留记录和标识。没有改写历史订单、价格表快照或已发布菜单。旧管理写接口不再修改历史入口的目标，新菜单选择器只列手工发布项。

## 开发交付

合入、部署和运行健康结果在此追加。此文档的自动检查通过不代表真实微信授权实测或 Van 产品验收完成。
