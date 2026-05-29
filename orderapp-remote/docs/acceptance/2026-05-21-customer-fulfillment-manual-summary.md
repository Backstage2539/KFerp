# 验收记录：客户履约手册总览补充

## 需求
- 在客户履约相关手册基础上，补充整体履约客户操作手册总结。
- 明确不同角色的操作权限边界。
- 明确不同功能的操作流程。
- 从新用户视角补充还需要手册化的内容，让客户和内部人员更容易上手。

## 文档变更
- `OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`

## 覆盖内容
- 新增“新用户先看”，把客户履约定义为从客户开通到追溯的全链路流程。
- 新增“相关手册怎么分工”，区分客户履约手册、客户门户手册和订单销售手册。
- 新增“角色权限速查”，覆盖管理员/老板、客户成功/实施、履约运营、商品/成本、仓库/生产、财务、履约客户外部用户、零售商城客户。
- 新增关键权限口径，说明 `customers.write`、`stock.write`、`stock.read`、`customer_processing.read`、`customer_processing.submit`、`orders.read`、`orders.write`、`settings.write` 的使用边界。
- 新增“功能流程总览”，覆盖新客户开通、商品和报价准备、客户侧下单、内部手工履约、Excel 导入、托管库存、订单承接、发货出库、月结对账和异常追溯。
- 新增“新用户上手清单”，按测试客户完整跑通一遍开通、下单、订单承接、发货、月结和操作日志。
- 新增“还需要继续手册化的内容”，列出首次开通检查表、模板选择决策树、客户侧小卡片、Excel 模板说明、价格和豆单排障、订单生命周期图、财务对账口径、权限申请说明和操作日志排查模板。

## 验证
- [x] 根目录手册和部署目录手册一致：`diff -u OP_MANUAL_CUSTOMER_FULFILLMENT.md orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- [x] 前端手册解析单测通过：`node --test src/lib/operation-manuals.test.js`
- [x] 手册治理后端守卫通过：`go test ./internal/interfaces/http/support -run TestOperationManual -count=1`
- [x] 文档格式检查通过：`git diff --check -- OP_MANUAL_CUSTOMER_FULFILLMENT.md orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`

## 结论
- 通过。客户履约手册已补充总览、角色权限、功能流程和新用户上手路径，且根目录与部署目录手册保持同步。
