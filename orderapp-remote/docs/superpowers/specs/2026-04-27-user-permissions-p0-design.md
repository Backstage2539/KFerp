# 用户权限 P0 设计

## 目标

- 保留现有 BasicAuth 管理员入口，作为全权限兜底。
- 保留移动端员工 Bearer 登录 session。
- 为内部员工建立角色、API 权限和 Vue shell 页面权限。
- 管理员可在 Vue/Vite 前端为员工分配角色。

## 范围

- P0 只做角色到权限、角色到页面可见性的粗粒度控制。
- 不做数据范围、审批流、字段级权限和组织层级继承。
- 未授权 API 返回 403；未登录返回 401。

## 角色

- admin：全权限。
- sales：订单、客户、商品查询。
- production：生产执行、生产查询、库存/物料/BOM 查看。
- warehouse：库存和物料维护。
- finance：订单、库存、商品和成本查看。
- product：商品、BOM、成本维护。
- system：权限、员工、设置、审计和需求流程维护。

## 权限面

- API 权限覆盖订单、客户、生产、库存、物料、BOM、产品设置、成本核算、系统设置、员工、审计、需求管理。
- 页面权限使用 `menu-ia.js` 的 view key，管理员返回 `allowed_views = null`，普通员工返回可访问 view key 列表。
- Vue shell 根据 `allowed_views` 过滤左侧菜单；直接访问未授权 view 显示无权访问。

## 验收

- 管理员打开 `/vue-shell?view=userPermissions` 可看到员工、角色和保存按钮。
- BasicAuth 管理员访问 `/api/auth/me` 返回 `basic_auth_admin=true`。
- 普通员工缺少权限时，对应 API 返回 403。
- 前端菜单只显示该员工授权页面。
