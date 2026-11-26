# PR-771 寄件回传批次筛选与一单多票

## 业务口径

- 员工在订单列表上传寄件列表，逐行查看候选、阻断原因和最终归属。“只看本次匹配订单”合并本批实时待选订单与已回填订单，按订单去重后分页。
- 仅允许在同次上传内人工选择一个订单的多个包裹。订单要满足发货条件；全部运单与成本在同一个事务内写入，整单一次发货与扣库。
- 手册入口为「订单列表 → 顺丰发货 → 回传使用说明」，单一源为 [寄件列表回传与实际运费确认](../OP_MANUAL_SHIPPING_RETURN.md)。

## 复现与验证

- 旧接口 GET 批次回传 404，旧解析器仅提示“单号或运费为空”。历史和新文件现在均能从模板识别缺失的应收金额列。
- 旧选择接口对多票一概返回 400。隔离 PostgreSQL + HTTP 的新用例覆盖同组反序并发一次生效、跨批拒绝、过期版本、库存阻断及晚失败整组回滚。
- 旧费用路径逐票重算 4.40 元和 7.60 元两票时，整单应收落到 9.60 元。新事务保存逐票成本，在固定附加费 2.00 元时确认整单应收 14.00 元。验证部分已收、已收清、包邮、收款不明、整组幂等及单票冲回保护。
- 开发数据库表升级为实际运费分录添加可空的 `shipping_return_group_id` 及局部索引。历史单票保留原有来源，分组费用可核对每一票的运单、揽件时间及重量。
- 使用虚构数据在 Chrome 实际 Vue 页面核对上传结果、跨页筛选、两票选择、确认后的 GET 回读和恢复全部订单。截图来自隔离页面，接口使用测试数据。
- 同批 2 票 1 单、其他批次、已回填、空匹配、客户/搜索条件交集、收件变化及未就绪状态由隔离 PostgreSQL 测试验证。新接口与批次筛选沿用员工权限。

[回传明细与候选](2026-11-25-shipping-return-report.png) · [同批两票确认](2026-11-25-shipping-return-picker.png)

## 验证命令

- `scripts/verify_kferp.sh changed`、`scripts/verify_kferp.sh backend`、`scripts/verify_kferp.sh frontend-tests`、`scripts/verify_kferp.sh frontend-build`。
- 在专用 PostgreSQL 上设置 `ORDERAPP_TEST_DATABASE_URL`，在 `orderapp-remote` 运行 `go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestShippingReturn|TestBillingDetailUsesOnlyFreightSourceWaybill|TestFreightActual' -count=1`。

## 开发环境验收

- 2026-11-25 已与员工新增客户校验合入 `develop` 并通过 `./deploy_orderapp.sh development` 部署。运行提交 `0c70d178839b43b679064ea5c155defb601c438e`。网页 2083 项测试、小程序 580 项测试、服务端及镜像内 Go 测试均通过。
- 开发应用运行且重启数为 0，数据库健康。公网登录页 200，匿名订单接口 401。两个批次入口对无效 ID 返回 400，对不存在的批次返回 404。部署后读取开发库，未发现已上传批次。多票确认与扣库验证使用隔离数据。
- 开发环境订单页资源、手册及 PR/DEV 表已核对。旧源码位于 `/opt/stacks/erp/orderapp.backup.deploy-20261125110222-0c70d178839b`，回滚镜像为 `kferp-orderapp-rollback:development-20261125110222-0c70d178839b`。
- 业务验收人 Van。在「订单列表 → 顺丰发货」选择「顺丰寄件列表」并上传，展开「本次回传明细」后即可核对并按本批匹配订单筛选。

## 生产验证

2026-11-26 已合入 main 并部署生产，运行提交 `1f5ada4cf65cd7d382efee3496eed41953de1331`。生产批次 6 有 11 行，其中 8 行未匹配、2 行待选择、1 行跳过。第 6、7 行指向同一张候选订单，批次筛选返回 1 张订单。核查使用只读查询。数据库备份 `/opt/stacks/erp-production/backups/pre-sf774-20261126210049.dump` 已校验。源码回滚点为 `/opt/stacks/erp-production/orderapp.backup.deploy-20261126210347-1f5ada4cf65c`，回滚镜像为 `kferp-orderapp-rollback:production-20261126210347-1f5ada4cf65c`。
