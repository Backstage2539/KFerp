# PR-734 订单详情复制增加客户名称与付款方式

## 验收范围

- 网页订单列表、客户工作台、客户门户、员工小程序和客户小程序的“复制订单详情”文本都包含“客户名称：”与“付款方式：”。
- 两个字段读取点击时的最新已保存订单快照；缺失值显示“未填写”。
- 原付款状态、商品规格/数量/单位、收件信息、权限校验和复制失败后的手动复制流程保持原有行为。
- 复制订单详情不会新建或修改订单，也不会自动发送消息。

## 当前交付状态

- 应用层格式化与 PostgreSQL 快照读取已更新，所有复用该接口的客户端会得到相同字段。
- 五份订单相关单一来源手册及网页订单列表/客户账户帮助提示已更新。
- RED：`go test ./internal/application/sales -run '^TestOrderCopyDetailsTextSavedFields$' -count=1` 在字段输出实现前失败，缺少客户名称与付款方式。
- GREEN：`go test ./internal/application/sales -run '^TestOrderCopyDetailsText' -count=1` 通过。
- API/PostgreSQL：`ORDERAPP_TEST_DATABASE_URL='postgres:///postgres?host=/tmp' go test ./internal/interfaces/http/sales -run '^TestOrderCopyDetails' -count=1` 通过，覆盖复制内容、保存快照与客户隔离。
- 前端：`node --test src/lib/order-details-copy.test.js src/lib/order-details-copy-runtime.test.js` 通过 4/4；`npm run build` 通过，含既有 chunk size 警告。
- 合并最新 `origin/develop@0ebc4c4ba` 后，`scripts/verify_kferp.sh changed`、`backend`、`frontend` 均通过；定向 API/PostgreSQL 用例再次通过。
- 浏览器与开发运行验收待部署后执行；Van 业务验收待完成。开发部署暂缓，因该 develop 基线包含 PR-733 后续工作，其隔离浏览器操作验收仍待完成。
