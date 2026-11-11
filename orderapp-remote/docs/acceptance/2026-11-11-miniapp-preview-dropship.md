# 管理员客户预览的代发入口

- 范围：按所选客户开通情况显示库存代发、无库存代发；进入后只读查看该客户的代发商品规格和收件地址，不提供录单、核价、提交或地址修改。
- 回归：普通客户保留原有代发下单流程；未开通场景不显示。预览不使用客户下单草稿，切换客户时丢弃过期响应。
- 安全：代发目录和地址仅以预览会话的客户身份读取；伪造客户编号不能切换客户，未开通代发不能读取目录；业务写请求继续拒绝，管理员撤权后会话立即失效。
- RED：新增首页测试时，已开通的代发入口因预览过滤被隐藏；新增接口测试时，目录及地址 GET 均返回 403。
- 自动验证：见小程序页面运行测试和 `go test ./internal/interfaces/http/customerportal ./internal/application/customerportal`。
- 开发交付：`640d7d6dc0f5b68c884af79acf6cf3807cfa0fa7`；服务器小程序 510 项和网页 1905 项测试、类型检查、构建、Go 全量及镜像内测试通过。开发服务运行、数据库健康，匿名需求接口 401，登录 200，需求记录可见。开发包 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`；服务回滚源 `/opt/stacks/erp/orderapp.backup.deploy-20261111121405-640d7d6dc0f5`，旧包 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev.backup-20261111122634-640d7d6dc0f5`。
- 手册：`OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`customer-portal-miniapp-test.md`；Vue 手册入口使用同一份文档。
- 业务验收：待用户使用喜沁咖啡预览确认。生产部署和微信小程序正式发布不包含在本次开发环境交付中。
