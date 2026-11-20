# PR-768 销售录单其他费用

验收人 Van。当前状态为待业务验收。

## 业务验收

1. ERP 销售录单输入商品 100 元、运费 10 元、其他费用 20 元及说明“定制包装”。应收 130 元，实际到账 130 元时货款及其他费用 120 元、运费 10 元。
2. 员工小程序填写同样费用，核对保存草稿、恢复、订单详情、清零、复制和新建下一单。购货留厂选客户仓并核对零运费及费用计入应收。
3. 已有收款时增加费用，先保存费用调整，再核对待收并按实际到账收款。已出账及已有真实收款的保护规则应生效。
4. 查看订单详情、复制文字、单张和组合销售单的预览、PDF 和图片及客户新账单。费用只随订单出账一次；历史单据保留原快照。

## 技术验证

- RED：网页金额及提交合同、小程序金额及明细、真实 PostgreSQL 订单 API、单据和账单快照新增测试在实现前均失败，按对应功能修复后重跑。
- GREEN：`scripts/verify_kferp.sh backend`；`scripts/verify_kferp.sh frontend`（2074 项测试通过、构建成功）；员工小程序 `npm run typecheck`、`npm test`（542 项测试通过）及 `npm run build:mp-weixin:development` 均通过。
- PostgreSQL 定向测试涵盖订单创建、旧客户端省略字段、清零、非法金额、出账锁定、已有实收、审计；单张及组合销售单版本、客户账单快照与财务抹零。另有员工小程序接口校验客户仓归属、类型和启用状态。
- 待确认订单费用暂存及旧版本、商品退款不退其他费用、客户禁止自行修改收费的定向测试通过。另行运行既有 `TestFulfillmentOrderConfirmationSameOrderPostgres` 时，其测试建单夹具缺少 stock_entries 表，未能通过；新增独立的真实 PostgreSQL 费用确认用例通过。
- 单张和组合销售单 PDF/PNG 已实际渲染及目检，长说明换行正常；新客户账单 PDF 已目检，示例商品 100 − 优惠 20 + 其他费用 7.50 = 87.50。
- 使用正式构建网页和独立模拟数据在 Chrome 检查桌面与 390px 手机网页；编辑单原合计 60 元，增加 12.35 元后变为 72.35 元，清空后恢复 60 元，说明、合计同屏且无横向溢出。
- 手册：OP_MANUAL_ORDER_SALES.md、OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md、OP_MANUAL_CUSTOMER_PORTAL.md、OP_MANUAL_CUSTOMER_FULFILLMENT.md。

## 开发环境部署

- 2026-11-20：功能分支已推送并合入 develop；`./deploy_orderapp.sh development` 部署提交 `1a09c4d620036a97d8e55ac9d49603283777fc1a`。服务器端 Vue 2074 项、小程序 542 项、Go 全套及镜像构建均成功。
- 开发服务已启动，数据库健康，登录 HTTP 200；匿名 API 401，认证录单页面和表单 HTTP 200。真实订单编辑接口含 `other_fee`、`other_fee_note`，历史行的新增列默认值分别为 0 和空说明。
- 开发包固定目录 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`，发布信息匹配部署提交，108 个清单文件校验通过。未上传微信。
- 上一版源码备份 `/opt/stacks/erp/orderapp.backup.deploy-20261120213530-1a09c4d62003`。
- PR/DEV 需求表已可查询，技术交付完成，待 Van 业务验收。
