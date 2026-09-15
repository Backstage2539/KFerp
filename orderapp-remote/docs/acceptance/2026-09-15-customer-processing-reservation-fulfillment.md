# PR-667 客户代加工在制预订与一件代发贯通验收记录

## 范围与口径

- 代加工商品必须归属当前客户、开启代加工标记，并拥有完整的默认已发布 BOM 与规格；客户别名独立维护，不覆盖商品档案。
- 代加工申请预览计算物料可支持的最大产量，提交事务重新计算并立即预订物料；超量整单拒绝。
- 一件代发按客户成品 FIFO、有效在制产出 FIFO 分配。代加工商品合计不足整单不创建；普通商品只保留未覆盖生产缺口。
- 入库按订单 FIFO 把在制预订转换为成品批次占用；取消释放，部分入库不提前完成，最终少产保留缺口。
- ERP 与小程序使用同一应用服务和数据库约束；客户、商品、BOM 规格与库存单位同时隔离。

## 自动验证矩阵

| 场景 | 预期 | 证据 |
| --- | --- | --- |
| 最大可生产量 | 上限数量可提交并预订全部 BOM 物料，上限加一预览和提交均失败 | `processing_bom_spec_identity_postgres_test.go`; `processing_request_authority_test.go` |
| 并发与原子性 | 申请和下单均在数据库事务内锁定重算；订单、请求、成品与在制分配一起提交或全部回滚 | `processing_requests.go`; `SubmitPreparedMiniDirectShipOrder`; PostgreSQL 用例 |
| 成品 80 + 在制 20 | 100 件订单按成品优先、申请 FIFO 补足；101 件代加工商品整单失败 | `TestMiniDirectShipPlannerUsesStockThenProcessingOutputFIFOAndBlocksOnlyProcessingShortage` |
| 普通商品缺货 | 不拦截整单，只把未被成品覆盖的数量留给生产 | 同上；customer fulfillment 应用测试 |
| 防止重复生产 | 订单行成品占用和未转换在制预订先从生产需求扣除 | `TestSalesOrderProductionDemandExcludesReservedStockAndProcessingOutput` |
| 完工入库 | 在制预订转订单成品批次占用并保存转换追溯；全部满足后待发货 | `customer_processing_output_reservation.go`; production 测试 |
| 取消与减产 | 取消订单释放成品/在制；有承诺的代加工工单禁止取消；最终少产记录缺口和受影响订单 | customer fulfillment/production 测试与审计断言 |
| 双端一致 | ERP 和小程序共用目录、预览、提交、进度与错误语义 | `TestERPProcessingRequestUsesSharedPreviewAndSubmitContract`; Vue/miniapp 测试 |

## 数据兼容与安全

- 商品 943 不做批量改写，继续通过现有代加工标记、客户归属与默认已发布 BOM 解析。
- 历史申请仅在物料预订存在、归属一致且预订量完整时贡献在制可用量。
- 新增订单行到代加工申请行的在制预订表和入库转换表，记录预订、转换、释放、缺口及最终批次。
- 申请提交、订单分配、取消释放、入库转换和减产缺口均进入操作日志。

## 发布记录

- 发布前完整校验：`./scripts/verify_kferp.sh all` 通过；Go 全包、Vue 1244 项测试及生产构建通过。
- 小程序校验：类型检查、42 个文件共 256 项测试、development 微信构建通过；16 个声明页面的四类产物文件及分享钩子完整。
- PostgreSQL 关键链路：物料上限与立即预订、80 成品 + 20 在制、101 整单拦截、入库转换、重复生产扣减、取消与客户隔离均通过。
- development：待完整验证后填写提交、部署时间、容器与接口/页面烟测。
- production：仅在 development 功能验证通过并合入 `main` 后填写；保留部署前镜像与回滚提交。
- Van 业务验收：自动和部署验证不替代真实客户业务验收，最终状态保留在 `REV-667`。
