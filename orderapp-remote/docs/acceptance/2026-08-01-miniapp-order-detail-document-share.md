# PR-572 小程序订单完整详情、单据导出分享与录单末尾新增商品验收记录

## 范围

- 员工小程序“查看订单”从概要卡片进入完整只读详情。
- 管理员查看全部订单，普通销售只查看本人订单；详情、生成和下载使用同一后端权限范围。
- 销售单和发货单均提供 PDF、PNG 正式文件，并可通过微信分享实际文件。
- “导出并微信分享”作为订单加载成功后的首个业务卡片显示在详情顶部。
- 员工小程序录单的“新增商品”入口位于商品明细末尾。
- 只合并和部署 development；production 不在本需求范围内。

## DEV 对照

- `DEV-572-EMPLOYEE-ORDER-DETAIL`：完成。
- `DEV-572-EMPLOYEE-ORDER-DOCUMENT-OUTPUT`：完成。
- `DEV-572-MINIAPP-WECHAT-FILE-SHARE`：完成。
- `DEV-572-DOCUMENT-SHARE-PANEL-TOP`：完成，分享区只移动模板位置，四类单据生成、版本、下载和微信分享逻辑不变。
- `DEV-572-ORDER-ENTRY-ADD-ITEM-POSITION`：完成。
- `DEV-572-DOCS-ACCEPTANCE-DELIVERY`：完成，development 集成、发布和冒烟证据已补齐。

## TDD RED 证据

- 支持合同：`go test ./internal/interfaces/http/support -run TestDev572MiniappOrderDetailDocumentShareContract -count=1`。
- RED 结果：失败，明确缺少员工订单详情页、列表跳转、详情路由、四类单据输出、微信文件分享以及相关手册标识。
- 后端、小程序和出库单 PNG 各自定向 RED 由对应测试输出补充。

## GREEN 证据

- 后端定向：application sales、PDF、Postgres sales、customerportal HTTP、sales HTTP 与 support 合同均通过。
- 后端完整：`cd orderapp-remote && go test ./... -count=1` 通过。
- 本机未配置 `ORDERAPP_TEST_DATABASE_URL` 时数据库集成用例按既有条件跳过；发布后使用 development 数据库中的唯一临时测试 schema 补跑 5 项文件回滚、PDF/PNG 成对资产、历史兼容和字段幂等用例，全部通过并自动删除测试 schema，未写真实订单数据。
- 小程序：21 个 Vitest 文件、132 项测试全部通过；`npm run typecheck` 通过；`npm run build:mp-weixin:development` 通过。
- 构建产物：`pages/employee-order-detail/employee-order-detail` 的 `.js/.json/.wxml/.wxss` 四文件齐全；“导出并微信分享”在编译后的 WXML 中只出现一次并排在订单概要之前；构建产物包含订单跳转、四类文件路径、`wx.shareFileMessage`、`wx.showShareImageMenu` 及降级逻辑。“新增商品”在编译后的商品循环之后、订单合计之前且仅出现一次。
- 视觉输出：`/tmp/kferp-delivery-note-pr572.png` 为 2480×9440 高清长图，已检查中文表头、长地址、36 行商品、长备注、签收栏和底部留白，无重叠、截断或分页裁切。
- 代码与文档：`scripts/verify_kferp.sh changed`、`git diff --check` 与 PR-572/旧出库单清理支持合同通过。
- 独立复核：鉴权状态透传、404 缺文件自愈、快递费文本、寄件人展示、正式图片下载路由、Echo PDF/PNG 路由冲突、部分文件写入回滚及真实文档版本文件名均已修复并复核关闭；无开放 P0-P2。真机微信分享仍按下方 Van 验收清单人工确认。

## 权限与操作日志

- 详情、下载和重复分享为只读，不新增业务操作日志。
- 无正式版本时由小程序触发的销售单、销售单图片或出库单正式生成，必须沿用现有生成服务和操作日志。
- 越权订单在详情、生成和下载入口统一返回不存在，不允许先读详情或文件再判断权限。

## 部署与冒烟

- 功能分支：`codex/miniapp-order-detail-share-20260801`。
- 功能提交：`a543c2dd3412f2d49f9dde86b8dacce8c2ff40bb`，development 远程预检通过且未修改运行环境。
- `origin/develop`：合并提交 `6e257b17cb96948c592a02a602f7e41494cb3f64`。
- development：已部署上述 `origin/develop`；外部 `https://dev.qacoohee.com/app/login` 返回 HTTP 200，订单详情及四类单据 GET/POST 未登录请求统一返回 401，容器日志无 `panic/fatal/error`。
- 数据库：正式 development schema 已存在 `delivery_note_documents.image_asset_id`；临时 schema 中 5 项 PostgreSQL 集成用例通过。
- 开发小程序：52 个清单文件复验通过，`RELEASE_INFO` 为 development、API 为 `https://dev.qacoohee.com/app`、提交为 `6e257b17`；固定目录 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`，上一版备份 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev.backup-20260801222026-6e257b17cb96`。
- 回滚点：源码 `/opt/stacks/erp/orderapp.backup.deploy-20260801221451-6e257b17cb96`；镜像 `kferp-orderapp-rollback:development-20260801221451-6e257b17cb96`。
- production：未部署；生产应用启动时间早于本次 development 切换且未重启。

## Van 验收清单

- [ ] 在开发版小程序进入“查看订单”，点击本人订单能看到完整详情。
- [ ] 用普通销售账号不能打开其他销售订单；管理员可以打开。
- [ ] 销售单 PDF、销售单图片、发货单 PDF、发货单图片均可分享给微信联系人。
- [ ] “导出并微信分享”位于订单详情顶部、订单概要之前，不需要滑到页面底部。
- [ ] 无正式单据时可生成后分享；只查看或重复分享不会产生新版本。
- [ ] 录单新增三条以上商品时，“新增商品”始终在最后一条商品下方。
