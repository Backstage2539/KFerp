# PR-704 小程序代发补库存与价格表体验

- 分支：codex/miniapp-stock-order-experience-20261019，基线 develop@758a2f50。
- DEV-800：购货留厂默认、独立补购草稿、补购往返刷新及请求状态。
- DEV-801：库存与无库存独立入口、全部库存概览、合并同规格数量及商品行提示。
- DEV-802：价格表复用客户会话、唯一复制按钮、手册与开发交付。
- 状态：自动验证进行中，随后开发部署及独立数据小程序交互验收；Van 产品验收待确认。生产、微信上传审核发布不在范围。

## RED 与修复

私有证据目录：/Users/yiiiple-work/KFerp-miniapp-stock-order-experience-20261019。

- 留厂无收件信息：真实旧 normalizer 返回 recipient_name required；retained-red.log。现在只对 product_order + customer_stock 豁免，普通发货及两种代发继续必填。
- 已登录客户重复登记：useBeanRegistration 新用例期望连续打开2表，实际0次；mini-red.log。真实PostgreSQL/HTTP同样返回401 mini_registration_required；price-postgres-red.log。现在复用有效客户会话及approved绑定，兼容历史客户account_type为空，不伪造registration_complete，不改变专属表隔离。
- 重复复制按钮、未拆分的首页入口及草稿相互覆盖：mini-extra-red.log。现在按客户/模式和独立补购上下文隔离，订单号下保留一个复制按钮。
- 原代码补购成功事件无人监听、onShow只刷新目录不重新报价；失效报价请求无法清理previewing。统一useCustomerOrderQuote管理刷新、取消、延迟报价及版本，异步Promise用例覆盖20变22、失败重试和晚到响应。
- 扩展真实数据库流程发现：成功购货重试时Prepare已返回历史记录，预览却尝试读取未加载的当前价格表，报“商品不在指定价格表当前有效数量档位中”。购货重试改用已保存商品价格快照，提交继续返回原订单，不重复购货。

## 验收范围

- 全部可用商品规格概览实时扣算本单数量，同规格多行合计；现货、备货后可用和其他代发已占用口径清晰。超量只在对应商品区域提示；非缺口错误继续明确显示。
- 补库存按钮常驻，未选商品和未填收件资料也可进入；缺口带缺口，无缺口空白，不覆盖普通购货草稿；返回后先更新目录再重新核价，失败保留草稿和重试入口。
- 开通库存代发时商品下单自动留厂并隐藏收件信息；无库存客户继续正常发货。独立模式入口仍由后端校验，仓库、价格、金额、发货门禁及操作日志保持。
- 已登录客户打开目录内价格表及直链不重复登记；无权、未选择客户和会话过期分别处理。未登录访客、员工范围、停用、撤回及其他客户价格表继续按原权限拒绝。

## 自动验证

- 小程序定向46项及全量362项通过，类型检查和development构建通过；后续修改完成后记录最终计数。
- Vue手册入口及全量1420项、生产构建通过；手册沿既有Vue入口读取唯一Markdown源文件。
- 真实隔离PostgreSQL：TestRetainedMiniPurchaseWithoutRecipient、TestBeanCenterRegistrationPostgres、TestPageEntryAccessPostgres及PR703回归通过。
- TestMiniRetainedTopupWithoutAddressRefreshesRightsAndCreatesOnlyGapDemand：原购20、需求22、无地址补购2、重试返回同订单、可预约22但实物0、生产需求仅22，断言通过。
- 完整Go、最终构建、开发运行及实际小程序证据在交付后补齐。数据库验证使用本机独立schema，不使用生产数据库。

## 手册与数据隔离

- OP_MANUAL_CUSTOMER_PORTAL.md：自动留厂、两入口、库存概览、补购返回和订单操作。
- OP_MANUAL_CUSTOMER_FULFILLMENT.md：网页选择模式与小程序独立入口、术语及补购操作。
- OP_MANUAL_BEAN_LIST_CENTER.md：有效客户会话、权限及失败处理；既有Vue手册入口同步读取。
- 实际交互使用新建独立验收SKU、自己的购货及代发单据，不占用已有喜沁商品库存、不修改既有单据；测试绑定与原咖啡豆价格表分开。交付前核对原库存及绑定保持。
