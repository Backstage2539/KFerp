# PR-572 小程序订单完整详情、单据导出分享与录单末尾新增商品验收记录

## 范围

- 员工小程序“查看订单”从概要卡片进入完整只读详情。
- 管理员查看全部订单，普通销售只查看本人订单；详情、生成和下载使用同一后端权限范围。
- 销售单和发货单均提供 PDF、PNG 正式文件，并可通过微信分享实际文件。
- 员工小程序录单的“新增商品”入口位于商品明细末尾。
- 只合并和部署 development；production 不在本需求范围内。

## DEV 对照

- `DEV-572-EMPLOYEE-ORDER-DETAIL`：完成。
- `DEV-572-EMPLOYEE-ORDER-DOCUMENT-OUTPUT`：完成。
- `DEV-572-MINIAPP-WECHAT-FILE-SHARE`：完成。
- `DEV-572-ORDER-ENTRY-ADD-ITEM-POSITION`：完成。
- `DEV-572-DOCS-ACCEPTANCE-DELIVERY`：完成，development 发布证据待本次集成后补齐。

## TDD RED 证据

- 支持合同：`go test ./internal/interfaces/http/support -run TestDev572MiniappOrderDetailDocumentShareContract -count=1`。
- RED 结果：失败，明确缺少员工订单详情页、列表跳转、详情路由、四类单据输出、微信文件分享以及相关手册标识。
- 后端、小程序和出库单 PNG 各自定向 RED 由对应测试输出补充。

## GREEN 证据

- 后端定向：application sales、PDF、Postgres sales、customerportal HTTP、sales HTTP 与 support 合同均通过。
- 后端完整：`cd orderapp-remote && go test ./... -count=1` 通过。
- 本机未配置 `ORDERAPP_TEST_DATABASE_URL`，3 项数据库清理/成对资产集成用例按既有条件跳过；合并前 development 远程预检必须在测试数据库中实际运行并补齐证据。
- 小程序：21 个 Vitest 文件、129 项测试全部通过；`npm run typecheck` 通过；`npm run build:mp-weixin:development` 通过。
- 构建产物：`pages/employee-order-detail/employee-order-detail` 的 `.js/.json/.wxml/.wxss` 四文件齐全；构建产物包含订单跳转、四类文件路径、`wx.shareFileMessage`、`wx.showShareImageMenu` 及降级逻辑。“新增商品”在编译后的商品循环之后、订单合计之前且仅出现一次。
- 视觉输出：`/tmp/kferp-delivery-note-pr572.png` 为 2480×9440 高清长图，已检查中文表头、长地址、36 行商品、长备注、签收栏和底部留白，无重叠、截断或分页裁切。
- 代码与文档：`scripts/verify_kferp.sh changed`、`git diff --check` 与 PR-572/旧出库单清理支持合同通过。
- 独立复核：鉴权状态透传、404 缺文件自愈、快递费文本、寄件人展示、正式图片下载路由、Echo PDF/PNG 路由冲突、部分文件写入回滚及真实文档版本文件名均已修复并复核关闭；无开放 P0-P2。真机微信分享仍按下方 Van 验收清单人工确认。

## 权限与操作日志

- 详情、下载和重复分享为只读，不新增业务操作日志。
- 无正式版本时由小程序触发的销售单、销售单图片或出库单正式生成，必须沿用现有生成服务和操作日志。
- 越权订单在详情、生成和下载入口统一返回不存在，不允许先读详情或文件再判断权限。

## 部署与冒烟

- 功能分支：`codex/miniapp-order-detail-share-20260801`。
- `origin/develop`：待合并。
- development 部署：待执行。
- production：未请求、不得部署。

## Van 验收清单

- [ ] 在开发版小程序进入“查看订单”，点击本人订单能看到完整详情。
- [ ] 用普通销售账号不能打开其他销售订单；管理员可以打开。
- [ ] 销售单 PDF、销售单图片、发货单 PDF、发货单图片均可分享给微信联系人。
- [ ] 无正式单据时可生成后分享；只查看或重复分享不会产生新版本。
- [ ] 录单新增三条以上商品时，“新增商品”始终在最后一条商品下方。
