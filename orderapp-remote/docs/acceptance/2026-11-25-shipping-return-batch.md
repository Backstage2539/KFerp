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
- 开发部署及运行版本另见 ACTIVE_REQUIREMENTS。Van 业务验收待完成。
