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

## 待开发环境实测

需在同一浏览器、数据和网络条件下取样 10 次，对比完整 JS 压缩传输量和冷启动首批数据显示中位耗时；预热后对产品选项接口取样 30 次并记录 P95。库存列表首进需核对不再请求全量 `/api/products`，同时保存请求数量、体积和耗时证据。生产环境发布不在本次范围。

## 回退

回退到本次前的应用版本即可恢复静态页面导入和库存全量产品读取；数据库没有结构变更，旧 `/api/products` 仍保留兼容行为。
