# PR-670 ERP 打开与切换速度优化实施记录（2026-09-21）

## 范围

本次只处理 ERP 页面加载、启动时无关选项请求、库存作业商品选择和慢请求观测。没有改变菜单地址、权限、客户/订单上下文、订单 SQL 业务结果或生产环境。

## 已实现

- `App.vue` 的业务视图改为 `defineAsyncComponent`。首次只加载公共壳和当前页面，切换时显示页面加载状态；异步文件失效提供重试入口。PDF 等重组件仍由实际页面按需加载。
- 启动阶段仅等待身份、权限和必要界面设置。客户、订单和视图预设在对应选择器获得焦点时加载；已有链接上下文保留 ID，并在需要时后台补齐显示选项。
- `GET /api/products/options` 使用名称、SKU 编号、条码或 ID 搜索与分页，默认 20 条、上限 100 条，返回 `rows/total/page/limit/has_next`。接口只读取商品识别字段；旧 `/api/products` 未修改。
- 库存作业首次进入只加载库存单据；打开新建/编辑单据时加载物料和仓库。商品选项输入延迟 250ms，支持下一页，选中后再读取 `/api/products/:id` 的完整 BOM 规格与单位。详情未完成时保存/提交被阻止，历史单据会回显并补齐商品详情。
- 前端 GET 请求按最终 URL 合并并发请求；超过 500ms 的 GET/写请求记录方法、路径、耗时、状态和返回条数。订单仓储层把列表、汇总及四类选项查询分别记录慢阶段。

## 自动验证

- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/sales` 通过。
- `node --check frontend-vue-shell/src/api/client.js` 通过。
- `npm run build` 通过；构建产物已拆出当前页面 chunk（例如 `OrdersView`、`StockOperationsView`、`ProductSettingsView`），PDF worker 保持独立大 chunk。
- API 单测覆盖 `/api/products/options` 的分页响应及 527 条总量场景；应用层单测覆盖 page/limit 归一化。

## 开发环境实测证据（2026-09-21）

- 已部署 `develop` 提交 `7fd53f01fa58b0c630c7661bb7ade3223024e0de`，开发登录冒烟 `https://dev.qacoohee.com/app/login` 返回 HTTP 200；生产环境未操作。
- `GET /api/products/options?page=1&limit=20` 返回 `total=563`、`rows=20`、`has_next=true`；第 29 页返回 3 条且 `has_next=false`，第 30 页返回 0 条；`q=咖啡` 返回 25 条总数。接口返回字段为识别所需的 `id/name/code/product_kind/active`（空值字段按 JSON 省略）。
- 预热后连续 30 次第一页请求均 HTTP 200，单页响应体 1,883 bytes；`time_starttransfer` P95 约 37.7ms，低于 300ms 目标。
- 登录后的 Vue shell HTML 只预加载公共入口、Vue 导出辅助和客户收件信息三个 JS 文件，使用 `curl --compressed` 实测合计约 54.5KB；订单页 chunk 与 PDF 预览 chunk 均未在首屏 HTML 中预加载。构建产物全量 JS（所有页面合计）仍约 846KB gzip，不能用全量数字代替首屏传输量。
- `GET /api/stock-documents` 返回 63 条单据、31,373 bytes；`StockEntriesView` 首次 `onMounted` 只调用单据列表，物料、仓库在新建/编辑打开时调用，代码中已无库存列表阶段的全量 `/api/products` 请求；商品选项只通过 `/api/products/options` 触发。

仍需在相同浏览器、数据和网络条件下取样 10 次，记录冷启动到菜单可操作及首批数据的中位耗时、页面切换 100ms 反馈，以及库存页面实际 Network 面板请求数量。生产环境发布不在本次范围。

## 回退

回退到本次前的应用版本即可恢复静态页面导入和库存全量产品读取；数据库没有结构变更，旧 `/api/products` 仍保留兼容行为。
