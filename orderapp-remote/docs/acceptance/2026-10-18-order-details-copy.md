# PR-701 订单详情文字复制

## 范围与实现

- 网页 ERP、客户工作台、客户我的订单，以及员工与普通客户小程序，均在列表和详情操作区增加“复制订单详情”。客户小程序继续使用展开详情；发货中心、加工单和现有“复制订单”保持原样。
- 服务端统一格式化，四个只读 GET 入口返回 `{ order_id, text }`；复用对应员工权限、客户归属检查。无迁移、无业务写入。
- 每次点击读取最新已保存订单，事务内读取完整明细及收件信息；不使用未保存表单或客户当前地址补齐历史数据。
- 文字保留订单日期、现有付款状态、多商品、小数数量、原数量单位及冻结规格，支持盒装、BOM、历史临时换算；缺失信息为“未填写”，作废订单带标识。不包含价格、金额和备注。
- 请求期间防重复点击并阻止行跳转；客户、登录身份或页面变化后丢弃迟到响应。网页剪贴板受限时显示可选文字，小程序失败提示重试。

## TDD 与自动化证据

- RED：新增 Go 类型/入口及网页、小程序复制逻辑尚不存在时，定向测试失败。原始日志：`/tmp/kferp-order-copy-red-go.log`、`/tmp/kferp-order-copy-red-vue.log`、`/tmp/kferp-order-copy-red-mini.log`。
- GREEN：格式化测试覆盖完整文字、全部行、小数、盒装、BOM/换算规格、历史缺失、付款状态和作废标识。
- HTTP 测试覆盖四个入口的最新数据、返回结构、员工范围、客户隔离、无权限/无登录拒绝；受限工作台在读取快照前拒绝越权请求。独立 PostgreSQL schema 验证所有明细、冻结数据和禁止当前地址回填。
- 网页辅助逻辑及实际 Vue 组件测试覆盖成功、失败、手动复制、重复点击、客户/身份变化和组件销毁；小程序辅助逻辑测试覆盖对应异步状态与剪贴板结果。
- `scripts/verify_kferp.sh all` 通过：Go 全量、Vue 1,404 项、Vite 构建及差异检查。最后权限校验调整后定向 Go HTTP/权限测试再次通过。
- 小程序全量 334 项通过，类型检查和开发构建通过；最终布局调整后再次构建通过。
- PostgreSQL 定向命令：`ORDERAPP_TEST_DATABASE_URL='postgres:///postgres?host=/tmp' go -C orderapp-remote test ./internal/interfaces/http/sales ./internal/interfaces/http/customerportal ./internal/interfaces/http/support -run 'Test(OrderCopyDetails|MiniOrderCopyDetails)' -count=1`。
- 全量 Go 未配置数据库环境时仍有原有数据库测试跳过；不将上述结果解释为全库集成验收。

## 本地交互与手册

- 使用实际复制按钮组件、模拟接口和独立浏览器，在 1280px/375px 验证真实剪贴板成功、受限后的完整文字选择、焦点、窄屏无横向溢出、切换客户后取消结果。不是线上业务流程验收。
- 证据：[桌面](assets/pr701/desktop.png)、[窄屏手动复制](assets/pr701/mobile-fallback.png)、[结果](assets/pr701/result.json)。
- 同步销售订单、客户账户、客户工作台、客户门户及员工小程序五份单一来源手册，以及 Vue 页面帮助；PR-701 / DEV-790～792 跟踪实现、客户端和交付。

## 交付状态

- 功能分支：`codex/order-details-copy-20261018`，起点 `origin/develop@491c4655`。
- 实现、本地验证、开发集成、部署、技术冒烟及小程序导出已完成。
- 生产环境、微信上传/审核/发布和 Van 业务验收不在本次执行范围。

## 开发交付回执

- 功能提交 `c8a8b16d`，含界面证据的已推送分支提交 `1c6f9e119a6aec270e9f10cbc190e1da490cc48b`；独立干净目录集成，部署时 `origin/develop` 为 `c223255bef6949f00ca91ae08e8fef1ca8417547`。
- 命令：`KFERP_MINIAPP_DEVELOPMENT_EXPORT_DIR=/Users/yiiiple-work/KFerp-order-copy-delivery-20261018/KFerp-miniapp-mp-weixin-dev ./deploy_orderapp.sh development`。
- 服务器再次通过 Vue 1,404 项、小程序 334 项、类型检查和构建、Go 全量与镜像内 Go 测试；发布脚本完成，外部登录页 HTTP 200。
- 开发服务 `erp_orderapp` running，镜像 `sha256:39ef1b5fa99d78dd94228e2c3582334da3febe803ec8314e6c9b55360028953d`；`erp_postgres` running/healthy。
- 只读技术检查：匿名根页面 303 跳登录；四个复制 API 匿名均 401；认证 Vue 订单入口 200；认证无效订单 ID 复制接口 400；销售手册含 PR-701，PR 处于 review，DEV-790/791/792 完成。仅更新交付跟踪状态并记录审计，没有读取或修改真实订单做业务验收。
- 原源码备份：`/opt/stacks/erp/orderapp.backup.deploy-20261018232849-c223255bef69`；回滚镜像：`kferp-orderapp-rollback:development-20261018232849-c223255bef69`。
- 开发小程序目录：`/Users/yiiiple-work/KFerp-order-copy-delivery-20261018/KFerp-miniapp-mp-weixin-dev`，`RELEASE_INFO` commit=`c223255b`、environment=`development`、API=`https://dev.qacoohee.com/app`，24 页面/96 个页面清单文件校验通过。
- 压缩包：`/Users/yiiiple-work/KFerp-order-copy-delivery-20261018/KFerp-miniapp-order-copy-dev-c223255b.zip`，CRC 通过；SHA-256：`d85753d347a8a4bf64f8ca216d311850ebcb2d26aa8b25adae1e945a0dcecb72`。
- RED/GREEN、构建及部署日志保存在交付目录 `evidence/`。回执及进度状态作为后续提交合并，运行功能版本仍为上述 `c223255b`；后续回执不重复部署。
