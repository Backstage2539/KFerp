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
- development：`develop@e0ec50ad18f7a8e2a1a77c29710fe17cb55ea4e8` 部署完成；旧源码 `/opt/stacks/erp/orderapp.backup.deploy-20260915233405-e0ec50ad18f7`，回滚镜像 `kferp-orderapp-rollback:development-20260915233405-e0ec50ad18f7`。应用运行且重启次数 0，PostgreSQL healthy，登录页 HTTP 200，新表与字段迁移存在；固定开发小程序包同步到 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`。
- development 功能验收：在专用开发夹具中打开客户商品完整配置，确认代加工标记、BOM 规格和默认 BOM；申请数量 1 显示最大可生产 3 且可提交，数量 4 显示超上限并明确整单不部分提交。未提交申请；临时打开的代加工标记已恢复，数据库确认商品标记为关闭、客户申请数为 0、产出预订数为 0。
- production：`main@0fb6d24e5b67f27c39f0f9a7523fc0fb3e97c4ba` 部署完成；旧源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20260916000423-0fb6d24e5b67`，回滚镜像 `kferp-orderapp-rollback:production-20260916000423-0fb6d24e5b67`。应用运行且重启次数 0，PostgreSQL healthy，登录页 HTTP 200，新表与字段迁移存在，近 15 分钟无 panic、fatal 或迁移失败；生产代加工页面只读加载成功，未修改商品、提交申请或创建订单。
- 小程序产物：development 与 production 固定目录分别同步；production 旧包保留在 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin.backup-20260916001147-0fb6d24e5b67`。服务器部署不等于微信发布，本次未执行上传、审核或发布。
- Van 业务验收：自动和部署验证不替代真实客户业务验收，最终状态保留在 `REV-667`。
