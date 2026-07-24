# PR-551 删除销售单追溯区块

日期：2026-07-24
环境：功能分支验证完成；development 已部署并完成 API 与浏览器冒烟
production：不部署，不改业务数据

## 用户需求

删除销售单页面和抽屉中的整块“销售单追溯”。

## 实现边界

- 删除“销售单追溯”“刷新追溯”、报价来源、生产来源和对应空状态。
- 删除销售单组件的追溯状态、格式化函数、专用样式及 `/api/orders/{id}/detail` 初始化请求。
- 订单详情继续保留报价来源和生产来源；详情 API、追溯快照和历史订单均不删除。
- 销售单预览、备注、客户信息、设置、PDF、图片、分享和历史版本不变。
- 本需求不新增业务写操作，因此不新增操作日志。

## RED

- `node --test src/lib/order-entry.test.js`：新测试在旧实现中发现“销售单追溯”面板和追溯详情请求，118/119 通过、目标用例失败。

## GREEN

- `node --test src/lib/order-entry.test.js`：119/119 通过；销售单组件不含追溯区块、追溯状态、格式化函数或详情追溯请求，订单详情追溯合同继续通过。
- `go test ./internal/interfaces/http/sales -run TestOrderAPIDetailAllowsCustomerWorkbenchBoundOrder -count=1`：通过；订单详情 API 继续返回报价来源和生产来源追溯。
- `go test ./internal/interfaces/http/support -run TestDev551RemoveSalesOrderTraceContracts -count=1`：通过。
- `scripts/verify_kferp.sh backend`：完整后端测试通过。
- `scripts/verify_kferp.sh frontend-build`：Vue/Vite production build 通过，402 modules。
- `scripts/verify_kferp.sh changed` 与 `git diff --check`：通过。

## 部署与冒烟

- 功能提交 `23151d30` 已推送到 `origin/codex/pr551-remove-sales-order-trace`；合并提交 `9d3fc26ba390b565ee4660cac4da85e18e15e181` 已推送到 `origin/develop`。
- 使用 `./deploy_orderapp.sh development` 部署；备份为 `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260724215239`。Docker 镜像构建内完整 `go test ./...` 通过。
- `erp_orderapp` 和 `erp_postgres` 正常运行，数据库 healthy，应用重启计数为 0；部署后日志未发现 panic、fatal、`conn busy` 或 error。
- 认证访问 development `/app/` 和订单 Vue 页面均返回 200；需求 API 返回并包含 `PR-551-REMOVE-SALES-ORDER-TRACE`。
- 浏览器打开单张销售单抽屉：抽屉唯一存在；“销售单追溯”“刷新追溯”“报价来源”“生产来源”均不存在；“销售单预览 V1”正常显示，控制台无错误。
- production 未部署、未写入；历史订单、追溯快照和销售单文件未修改。
