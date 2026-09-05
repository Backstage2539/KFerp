# PR-628 商品与物料显式客户关联开发环境验收

## 范围

- 分支：`codex/explicit-customer-catalog-links`
- 目标环境：development
- 生产环境：不发布、不改数据
- 验收客户：芬纳咖啡（客户 ID 74）及独立隔离客户

## 自动验证

- TDD RED：商品创建载荷缺少显式归属、客户商品仍依赖旧公共开关、物料客户关联模块不存在。
- GREEN：全部 Go 测试、全部 Vue Node 测试、Vite 构建和统一发布检查。
- PostgreSQL：商品重复关联幂等且专属商品拒绝跨客户关联；物料创建可同时关联两个客户，第三个客户不可见，停用后立即退出客户范围，操作日志存在。

## 开发环境数据纠正门禁

仅纠正商品 1073 `test` 与 1074 `test2`。执行前必须同时满足：

1. 商品有效、`customer_id=0` 且 `visibility=public`。
2. 没有商品产出 BOM 或 BOM 组件依赖。
3. 没有订单行引用。
4. 没有客户商品引用。

满足后在一个事务中改为 `customer_id=74`、`visibility=customer_only`，新增有效 `product_customer_references`，供料方式为 `factory`，并写商品归属纠正与客户引用操作日志。任一前置条件不满足则整笔中止。重复执行时只允许确认已处于目标状态，不重复新增有效引用或日志。

## 开发环境结果

- **结论：通过。** 最终代码已合入 `develop` 并发布到 development；production 未发布、未改数据。
- 发布门禁：Web 1072/1072、微信端 220/220、全部 Go 测试、Vite 与 mp-weixin 构建全部通过；容器、数据库、内部 HTTP 和 `https://dev.qacoohee.com/app/login` 健康。
- 商品创建：显式工厂商品 1075 与显式芬纳商品 1076 均创建成功，未出现 SQLSTATE 42P08。1076 为 `customer_id=74`、`customer_only`，引用供料方式为 `customer`。
- 商品关联：公共商品 1075 两次提交均返回引用 22，落库只有一条；客户显示名为 `PR628芬纳公共商品B`。把芬纳专属商品 1076 关联给客户 298 返回 400，拒绝跨货主关联。
- 商品隔离：客户 74 的显式范围包含 1073、1074、1075、1076；客户 298/299 不能取得这些芬纳专属商品。页面中客户 C 搜索 `PR628` 为 0 款。
- 物料关联：物料 75 同时关联客户 74 和 298；两次关联客户 298 均返回引用 2，落库只有一条。停用引用 1 后客户 74 查询为空，恢复后重新可见；客户 298 可见，未关联客户 299 查询为空。
- 页面：工厂商品页展示全部商品、档案归属筛选和客户关联操作；新建商品必须先选档案归属。工厂物料页展示客户关联筛选/列/操作；新建物料必须先选仅工厂使用或关联客户。客户页标题和顶部当前视图均按客户名称展示。
- 1073 `test`、1074 `test2`：执行前订单行、客户引用和已配置 BOM 版本均为 0。系统为两者自动生成的空 BOM 骨架没有规格、组件、路线、发布版本或业务绑定，修正时保留；事务把两者改为芬纳 `customer_only`，新增引用 19/20，供料方式 `factory`，并写审计 8388-8391。原始数据备份在 `/opt/stacks/erp/backups/pr628-products-1073-1074-before-20260905114244`。
- 验收数据：客户 74 `芬纳咖啡`、客户 298 `PR628隔离客户B`、客户 299 `PR628隔离客户C`；商品 1073-1076、物料 75 及其引用全部保留供复查。
- 服务器证据：`/opt/stacks/erp/orderapp_data/acceptance/pr628/`，包含请求结果、越界拒绝、数据库 CSV、审计、发布日志及汇总状态；稳定发布日志另存为 `deploy-final.log`。
- 页面截图：`pr628-product-factory-associations.png`、`pr628-product-create-ownership.png`、`pr628-product-customer-fenna.png`、`pr628-product-isolation-customer-c.png`、`pr628-material-factory-associations.png`、`pr628-material-create-scope.png`、`pr628-material-customer-fenna.png`、`pr628-material-isolation-customer-c.png`。
- 首次最终发布在容器启动阶段因服务器历史构建缓存占满磁盘而被健康门禁自动回滚。清理未使用缓存和过期源码备份后，磁盘占用降至约 80%，再次完整发布通过；数据库卷、当前运行镜像及最近两份回滚源码未删除。
