# PR-660 阶梯档位固定价与录单真实分类

日期：2026-09-13。功能分支：`codex/price-tier-fixed-order-category-20260913`。目标：合入 `develop` 并部署 development，完成技术验收后合入 `main` 并部署 production。业务验收人：Van。

## 交付行为

- 阶梯模板的每个档位可以选择“价格计算模板”或“固定价”。模板只保存定价方式；固定价档位清空价格计算模板，计算档位继续强制选择有效模板。旧模板按计算档位兼容读取。
- 编辑、改名和排序沿用原档位 ID。价格表草稿按“规格 × 阶梯模板 × 档位”保存固定价；切换模板或删除档位后失效映射不进入生成与发布。
- 选中商品规格后，在规格下逐档填写固定价。平铺价格行自动引用，显示“引用规格固定价”；单行修改显示“人工调整”，撤销后恢复规格固定价。客户历史报价不覆盖本次明确配置的固定价档位。
- 固定价缺失时规格区和平铺价格行标红并阻止发布。发布快照保存 `tier_pricing_mode=fixed_price`、`fixed_unit_price` 和最终价；计算档位继续保存 Pricing Rule 快照。历史快照缺少定价方式时按计算档位读取。
- 录单商品分类来自当前所选已发布价格表及 publication ID，并按商品分组顺序展示。生产当前分类“咖啡挂耳”和“好玩的产品”可以显示和筛选；商品标签显示真实发布分类。动态资料加载失败时才回退旧商品形态筛选。
- 价格表分类为录单展示与筛选权威。旧 `product_kind` 保留供历史流程兼容，不用于覆盖发布分类；本次不修补生产商品数据，也不改写历史价格表或订单。
- 阶梯模板保存、更新和删除继续使用既有写接口及操作日志。

## TDD 与自动验证

- RED：混合定价模板、旧模板兼容、固定价发布校验、档位 ID 稳定、草稿恢复、客户报价优先级、平铺行调整/撤销、发布分类筛选与标签测试先因缺少字段、校验分支、草稿映射和分类解析失败。
- 定向 GREEN：catalog、costing 的 Go 应用/仓储/HTTP 测试通过；价格表与录单相关 Vue 工具和页面接线测试共 `480/480` 通过。
- 完整门禁：`scripts/verify_kferp.sh all` 通过；Go 全包通过，Vue 共 `1229/1229` 项断言通过，Vite 生产构建通过，仅保留既有分块体积提示。

## 接口与兼容验收

1. `POST/PUT /api/price-tier-templates` 接受档位 `pricing_mode`；固定价档位不要求 `pricing_rule_id`，计算档位仍严格校验，旧客户端未传时默认为 `pricing_rule`。
2. 已发布价格表的 `price_rows` 为固定档位保存 `tier_pricing_mode` 与 `fixed_unit_price`，计算档位继续保存 Pricing Rule 版本和配置快照；固定价缺失时发布失败。
3. 阶梯模板已有档位按原 ID 更新，新档位才分配新 ID；删除或跨模板 ID 不会被错误复用。
4. `GET /api/order/form` 沿用现有分类模板、价格表版本与 publication 字段，前端不增加录单接口请求。
5. 商品 940 即使旧 `product_kind=roasted`，只要位于 publication 62 的“咖啡挂耳”，就归入“咖啡挂耳”；商品 941“咖啡果皮茶”位于 publication 63 时归入“好玩的产品”。两者选择规格后读取对应发布价格。

## 手册与页面帮助

- 价格表：`orderapp-remote/docs/OP_MANUAL_COSTING.md` 已补充档位定价方式、规格固定价、必填校验、手工调整/撤销、刷新和历史兼容。
- 录单：`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md` 已改为真实发布分类，并补充异常处理和价格表权威边界。
- Vue 页面内的阶梯模板、规格固定价和录单分类帮助文本同步更新。

## Development 与 Production 证据

- Development 最终运行提交 `6304bc91059352a6ca2810ed2592fd6ac713536f`。`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development` 退出 0；登录页和外部 Vue 地址返回 200，`erp_orderapp` 状态 running、重启次数 0，PostgreSQL healthy。
- Development 源码备份 `/opt/stacks/erp/orderapp.backup.deploy-20260913232913-6304bc910593`；回滚镜像 `kferp-orderapp-rollback:development-20260913232913-6304bc910593`。
- Development API 只读烟测：需求接口能读取 PR-660；阶梯模板接口返回 `pricing_mode`；录单表单接口返回 200。
- Development 浏览器验收：阶梯模板抽屉逐档显示“价格计算模板 / 固定价”，说明固定金额在商品规格下填写；录单商品下拉按当前发布价格表显示“咖啡生豆”真实分类和发布版本，未出现旧“熟豆 / 挂耳”硬编码标签；浏览器控制台无错误。开发数据当前没有已发布的挂耳或好玩的产品候选，因此这两类用生产结构自动测试和 production 只读页面继续核对。
- Production 已运行提交 `d1db829aeca709435129a3a37b3a84eab575ef3a`。`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production` 退出 0；登录页返回 200，`erp_prod_orderapp` 状态 running、重启次数 0，PostgreSQL healthy，发布后五分钟应用日志未出现 panic、fatal 或 error。
- Production 源码备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20260913233941-d1db829aeca7`；回滚镜像 `kferp-orderapp-rollback:production-20260913233941-d1db829aeca7`。
- Production 录单商品下拉显示“全部、烘焙咖啡豆、咖啡挂耳、速溶咖啡、好玩的产品、咖啡生豆”，分类顺序跟随当前商品分组配置；页面列出对应当前发布价格表和版本。
- “咖啡挂耳”筛选仅留下挂耳商品；“黑巧炸弹挂耳-盒装”显示 3 个可售规格，选择“挂耳红色盒装”后读取 V3.0.12，自动价 25.18 元/盒，并显示 1-10盒、10盒+ 两档。“好玩的产品”筛选仅留下“咖啡果皮茶”，选择 1Kg 后读取 V3.0.13，自动价 170 元/kg。两个商品标签均为真实发布分类，页面控制台无错误。
- 生产烟测只读取现有已发布价格表与录单候选，不新建、修改或发布正式业务价格。
- 录单验收仅在浏览器内选择商品和规格，没有点击“保存订单”，没有产生正式订单或价格写入。
- 技术验证由 Codex 完成；Van 的业务验收不代签。
