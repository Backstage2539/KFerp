# PR-565 小程序录单商品规格与收货信息验收记录

## 验收范围

- 小程序录单页明确展示商品、规格、数量及当前明细摘要。
- 商品名称与规格分离，规格只能从所选商品的具体 SKU 中选择。
- 正式环境历史商品缺少商品族快照时，按父商品、SKU 和规格字段回退重建。
- 选择客户后自动带入可编辑的收货人、电话、单位和地址。

## 自动化证据

- 后端定向测试：`go test ./internal/interfaces/http/customerportal ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales -count=1`，通过。
- 后端完整回归：`go test ./... -count=1`，全部包通过，包含架构分层、支持模块和 API 测试。
- 小程序单元测试：`npm test`，16 个测试文件、77 项测试全部通过；本需求 `employeeOrder.test.ts` 3 项通过。
- 小程序类型检查：`npm run typecheck`，通过。
- 微信小程序构建：`npm run build:mp-weixin`，通过并生成 `dist/build/mp-weixin`。

## 环境验收

- Development：待部署后记录提交、接口商品族数量、收货字段结构和静态资源状态。
- Production：待部署后记录提交、接口商品族数量、收货字段结构和静态资源状态。
- 所有线上验证只读取表单主数据与页面资源，不自动创建真实订单，不输出客户敏感信息。

## 操作手册

- `docs/OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md` 已更新客户回填、商品/规格选择顺序、字段核对和常见异常。

## 结论

- 待自动化测试及双环境只读冒烟完成后更新。
