# PR-704 小程序代发补库存与价格表体验

- 分支：codex/miniapp-stock-order-experience-20261019，基线 develop@758a2f50。
- DEV-800：购货留厂默认、独立补购草稿、补购往返刷新及请求状态。
- DEV-801：库存与无库存独立入口、全部库存概览、合并同规格数量及商品行提示。
- DEV-802：价格表复用客户会话、唯一复制按钮、手册与开发交付。
- 状态：实现、自动验证、独立开发数据客户交互验收通过；PR review，DEV-800/801/802 done，Van 产品验收待确认。开发部署证据见下。生产、微信上传审核发布不在范围。

## RED 与修复

私有证据目录：/Users/yiiiple-work/KFerp-miniapp-stock-order-experience-20261019。

- 留厂无收件信息：真实旧 normalizer 返回 recipient_name required；retained-red.log。现在只对 product_order + customer_stock 豁免，普通发货及两种代发继续必填。
- 已登录客户重复登记：useBeanRegistration 新用例期望连续打开2表，实际0次；mini-red.log。真实PostgreSQL/HTTP同样返回401 mini_registration_required；price-postgres-red.log。现在复用有效客户会话及approved绑定，兼容历史客户account_type为空，不伪造registration_complete，不改变专属表隔离。
- 重复复制按钮、未拆分的首页入口及草稿相互覆盖：mini-extra-red.log。现在按客户/模式和独立补购上下文隔离，订单号下保留一个复制按钮。
- 原代码补购成功事件无人监听、onShow只刷新目录不重新报价；失效报价请求无法清理previewing。统一useCustomerOrderQuote管理刷新、取消、延迟报价及版本，异步Promise用例覆盖20变22、失败重试和晚到响应。
- 扩展真实数据库流程发现：成功购货重试时Prepare已返回历史记录，预览却尝试读取未加载的当前价格表，报“商品不在指定价格表当前有效数量档位中”。购货重试改用已保存商品价格快照，提交继续返回原订单，不重复购货。
- 实际小程序发现：提交代发22袋成功后，后台已占用，但空白新单仍显示22袋。保留 mini-red-after-submit-stale22.jpg 及 after-submit-red.log；提交成功后清理报价并重新获取目录。修复后22袋安排12袋，页面和API均立即显示10袋，mini-green-after-submit-stock10.jpg；提交失败继续保留原草稿及请求标识。

## 验收范围

- 全部可用商品规格概览实时扣算本单数量，同规格多行合计；现货、备货后可用和其他代发已占用口径清晰。超量只在对应商品区域提示；非缺口错误继续明确显示。
- 补库存按钮常驻，未选商品和未填收件资料也可进入；缺口带缺口，无缺口空白，不覆盖普通购货草稿；返回后先更新目录再重新核价，失败保留草稿和重试入口。
- 开通库存代发时商品下单自动留厂并隐藏收件信息；无库存客户继续正常发货。独立模式入口仍由后端校验，仓库、价格、金额、发货门禁及操作日志保持。
- 已登录客户打开目录内价格表及直链不重复登记；无权、未选择客户和会话过期分别处理。未登录访客、员工范围、停用、撤回及其他客户价格表继续按原权限拒绝。

## 自动验证

- 小程序定向46项、提交后刷新6项及最终全量364项通过；类型检查、development构建通过。mini-full-final.log、mini-typecheck-final.log、mini-build-final.log。
- Vue手册入口及全量1420项、生产构建通过；手册沿既有Vue入口读取唯一Markdown源文件。
- 真实隔离PostgreSQL：TestRetainedMiniPurchaseWithoutRecipient、TestBeanCenterRegistrationPostgres、TestPageEntryAccessPostgres及PR703回归通过。
- TestMiniRetainedTopupWithoutAddressRefreshesRightsAndCreatesOnlyGapDemand：原购20、需求22、无地址补购2、重试返回同订单、可预约22但实物0、生产需求仅22，断言通过。
- 完整Go通过：backend-final.log；数据库验证使用本机独立schema，不使用生产数据库。

## 客户登录实际交互与账单

- 微信开发者工具独立项目使用现有喜沁客户会话。空白代发可打开空白“下单库存”，无交付方式选择、无收件必填；手动返回正常刷新。原用户开发者工具项目及原草稿未修改。
- 专用SKU1156/454g，原购20袋（1633），代发22袋只显示一条缺口提示；补购页仅2袋，商品款2元，无收件输入；客户提交成功生成1634后自动返回原草稿22袋，地址、备注保留，可用量变22，旧缺口消失。mini-topup-return-refreshed22.jpg。
- 22袋代发1635：商品款0、服务费14元、运费28元。实物0时发货接口400，明确“客户库存尚未实际入库，不能发货”。取消后恢复22袋且服务费与运费冲回。
- 重新代发12袋1636：商品款0、服务费9元、运费18元，计费重量5648g；提交后剩余10袋。货款只在20+2购货单记录22元。API生产需求22袋只关联来源1633/1634，两个代发单没有重复生成生产需求。
- 每张订单号下只有一个“复制订单详情”，保留销售单、出库单。mini-one-copy-button.jpg。
- 同一登录会话连续打开标准表和一件代发表，直接显示商品与价格，无再次登录/登记弹层。入口文案“可直接查看”；mini-price-session-table1.jpg、mini-price-session-table2.jpg、mini-price-center-authenticated.jpg。
- 专用代发单均取消，专用来源购货单作废，独立价格绑定解除。原SKU1155可安排数量20袋、原咖啡豆绑定137/138和所有原账单行逐项一致，应付恢复2976元，专用SKU生产需求0、实物库存0；未消耗原手工验收库存。保留取消/作废操作日志及历史，不清除审计。
- 证据：verify-ui-final.py、cleanup-independent-fixture.py、pr704-final-trace.private.json及相应API响应保存在私有证据目录；不包含账号密码进入仓库。

## 开发交付

- 首次集成开发提交 d3ffebd373e746603191aee52cd220adce612e2b，部署成功，24页面/96清单文件导出通过。备份 /opt/stacks/erp/orderapp.backup.deploy-20261019110550-d3ffebd373e7。
- 最终行为版本及开发包：be7338a1646152bf2670ec4d846bc50068ca5e03，已推送功能分支、合入develop并部署development。开发包固定目录为私有证据目录下 KFerp-miniapp-mp-weixin-dev。
- 发布命令：`KFERP_MINIAPP_DEVELOPMENT_EXPORT_DIR=/Users/yiiiple-work/KFerp-miniapp-stock-order-experience-20261019/KFerp-miniapp-mp-weixin-dev ./deploy_orderapp.sh development`。
- 最终备份：/opt/stacks/erp/orderapp.backup.deploy-20261019113916-be7338a16461；回滚镜像：kferp-orderapp-rollback:development-20261019113916-be7338a16461。
- 最终24页面、96清单文件验证通过；导出包223个代码文件与实际DevTools验收项目逐字节一致。ZIP SHA256：6eb9854e14feae218ac923c70fbe8f681bb0ea3c0d850622e9db0a8ab97ccb11。
- 运行检查：开发应用running、数据库healthy；匿名/app/返回303至orders，受保护API匿名401，认证Vue履约页200；PR-704 review、DEV-800/801/802 done。生产应用启动时间保持2026-10-18T16:37:45Z，未做生产发布。
- 最终导出ZIP：私有证据目录下 KFerp-PR704-miniapp-development-20261019.zip；详细发布与健康信息在delivery-verification.json、runtime-health.json、runtime-final.log。
- 本文交付证据在部署后作纯文档补记；运行代码与开发包均对应上述be7338a1版本，文档补记无需重复部署。

## 手册与数据隔离

- OP_MANUAL_CUSTOMER_PORTAL.md：自动留厂、两入口、库存概览、补购返回和订单操作。
- OP_MANUAL_CUSTOMER_FULFILLMENT.md：网页选择模式与小程序独立入口、术语及补购操作。
- OP_MANUAL_BEAN_LIST_CENTER.md：有效客户会话、权限及失败处理；既有Vue手册入口同步读取。
- 实际交互使用新建独立验收SKU、自己的购货及代发单据，不占用已有喜沁商品库存、不修改既有单据；测试绑定与原咖啡豆价格表分开。交付前核对原库存及绑定保持。

## 验收期间追加调整

- Van 要求库存概览按钮简化为“商品下单”；点击时自动保存原代发草稿，沿用仅预填缺口及补购返回流程。最小页面用例先RED（button-label-red.log），改文案后GREEN；手册同步。

- 追加验证：库存为空且未选商品/无地址、已选商品剩余0两种状态，直接渲染实际库存概览模板，“商品下单”均可见且可用（verify-empty-stock-button.cjs、empty-stock-button-green.log）。实际DevTools点击改名按钮进入仅2袋的补购页，手动返回保留原22袋及地址；未提交该草稿，未增加现有客户订单。截图mini-product-order-auto-draft-return.jpg。
- 网页复核：共用地址簿正常，曜石454g/12袋预览商品款0、服务费9元、运费18元，总计27元，未提交或占用现有库存。web-shared-address-book.jpg、web-stock12-fees.jpg。
- 代码复核：请求失效/卸载后的回包隔离、提交失败保留原标识、成功后刷新占用库存、模式与客户草稿隔离、只放宽留厂收件校验、价格表专属/停用/撤回门禁均保留。没有待处理的实现问题；产品验收由Van确认。
