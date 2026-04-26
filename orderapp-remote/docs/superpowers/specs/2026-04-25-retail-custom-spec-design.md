# Retail Custom Spec Design

## Requirement

零售录单需要支持自定义克数。用户可以继续选择商品已有零售价规格，也可以输入任意正整数克数。保存订单时，订单明细必须保存真实规格，例如 `300g`、`380g`、`1000g`。

## Pricing Rules

- 仅零售订单显示自定义克数入口。
- 商品有精确零售价的规格继续显示为快捷规格。
- 自定义克数如果刚好命中精确零售价，使用精确价格。
- 自定义克数没有精确零售价时，按 `227g` 零售价换算：
  - `ceil(retail_price_227g * spec_g / 227)`
- 如果商品没有可用 `227g` 零售价，前端显示价格为 `0` 并提示维护商品零售价；后端保持现有校验和计算兜底。

## Frontend Architecture

录单页历史上由 `templates/order.html` 实现。根据工作区规则，本需求不能继续在 HTML 模板上加用户功能。

本次把 `录单` 迁移为 `frontend-vue-shell` 内部 Vue 页面：

- `/order` 的 GET 入口重定向到 `/vue-shell?view=order`。
- `frontend-vue-shell/src/App.vue` 的 `order` 菜单项改为内部 Vue 页面。
- 新增 `frontend-vue-shell/src/views/OrderEntryView.vue`。
- 新增 `frontend-vue-shell/src/lib/order-entry.js`，放置规格、价格、payload 计算等纯函数。

旧 `templates/order.html` 已在 Vue/Vite 迁移完成后删除，`/order` 仅作为跳转到 Vue 录单页的兼容入口。

## Backend API

新增 JSON API：

- `GET /api/order/form`
  - 返回日期、客户、来源、订单类型、付款状态、发货状态、商品及商品零售价格。
  - 支持 `edit_id` 参数，为后续编辑复用提供数据。
- `POST /api/order`
  - 接收 Vue 提交的订单 payload。
  - 复用现有 `sales.SaveOrder` 流程。
  - 返回 `order_id`、`order_no`、`edited`、`redirect_url`。

Vue 提交 payload 时仍映射到现有 `CreateOrderRequest` 字段，避免重写订单保存业务逻辑。

## UI Behavior

- 订单类型选择为“零售”时，规格选择区域显示：
  - 该商品已有零售价规格
  - `自定义克数`
- 选择 `自定义克数` 后，显示 `克数(g)` 输入框。
- 输入必须是正整数。
- 价格、单行总价、订单总价实时计算。
- 非零售订单继续使用批发规格和阶梯价。

## Tests

- 单元测试：
  - `order-entry.js` 自定义克数价格按 `227g` 向上取整。
  - 自定义克数命中精确规格时使用精确价格。
  - `buildOrderPayload` 保存真实 `spec`。
- Go/API 测试：
  - `GET /api/order/form` 返回 `products` 和 `retail_specs`。
  - `POST /api/order` 提交零售 `300g` 后，数据库明细保存 `spec='300g'`，金额按 `227g` 换算。
  - `/order` GET 入口重定向到 Vue 录单页。
