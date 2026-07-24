# PR-551 删除销售单追溯区块

日期：2026-07-24
环境：功能分支验证中；development 待部署
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

- development：待部署。
- production：不部署。
