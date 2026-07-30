# PR-537 KMM 阶梯售价与挂耳通用价格表录单兼容验收

- 日期：2026-07-15
- 状态：代码、开发环境部署、KMM 数据导入、录单与财务烟测均已完成；生产环境未部署、未写入。
- 需求：`PR-537-KMM-PRICING-ORDER-COMPAT`
- 数据来源：服务器 `/data/kmm.xlsx`

## 问题与目标行为

- PR-415 已让新挂耳价格表进入通用商品价格表，但录单链路仍曾把 `product_kind=drip_bag` 强制映射到旧 `list_type=drip`，导致按新架构发布的挂耳派生袋/盒 SKU 无法自动取价。
- 新挂耳价格统一使用 `commercial price_rows`。袋、盒分别是派生子 SKU，必须分别固化销售规格、价格单位、库存单位和换算，不能再由父商品或旧挂耳模板代替。
- ERP 录单只允许 `publication_purpose=factory_supply` 作为价格来源；`customer_resale` 只用于客户转售分享，即使显式传入发布 ID 也不能进入订单价格快照。
- `commercial` 允许部分发布，因此同一所有者的历史发布按新到旧、以每个派生 SKU 身份为覆盖粒度合并：平铺行有效 `sku_id` 优先于父 `product_id`，同一 SKU 只取最近一次包含它的快照，未出现在新版的其他 SKU 可继续取最近历史快照；新版已包含某 SKU 但价格为空时阻断旧价回退。
- 录单优先使用行上已冻结或当前适用的 `commercial` 发布快照；只有该 SKU 不存在适用的 commercial 发布时才只读回退历史 `drip` 快照。commercial 发布存在但价格行缺失或阶梯无效时直接报错，不用旧价掩盖新表缺陷。系统不恢复挂耳专用模板、专用接口或新 `drip` 发布入口。
- 熟豆与挂耳袋/盒只在数量命中已发布档位上下界时自动取价。低于起订量、高于有限最高量或落在档位空隙时自动价为空并阻断保存，不再猜测最低档或末档；显式盒装档存在时，盒数越界也不得折算袋数回退。
- 订单行冻结实际命中的价格表发布 ID、版本、SKU、阶梯、单价、价格单位和库存换算，历史订单不受后续商品单位或价格配置变化影响。

## KMM 数据边界

- 本批数据只允许导入开发环境。执行前必须保留开发数据库备份；生产数据库、生产容器和生产入口不在本次数据写入范围。
- 物料价格只取“物料成本”页能够唯一匹配的 A 列物料名称与 E 列价格；BOM 关系和供应阶梯售价只取工作簿可明确证明的原始关系。
- 导入按单条记录继续并保持幂等，报告分列成功、跳过、失败和待人工确认。未匹配、名称歧义、空白或零价格、配方不完整时不猜测、不补 0 元、不发布，修正来源后再继续导入。
- 物料、BOM 版本、Pricing Rule、阶梯模板、价格表发布均通过正式 API 写入，并在操作日志中保留执行人、对象和变更结果。

## 代码合同

- ERP 录单的 Vue 辅助函数和 sales 仓储专用解析把新挂耳派生 SKU 归入 `commercial`；共享 `ListTypeForProductKind` 继续保留历史 `drip` 映射，避免改变客户门户等尚未迁移的旧链路。生豆仍使用 `green`，普通商品仍使用 `commercial`。
- 订单表单把挂耳纳入通用 `commercial` 发布价格候选，并保留袋、盒等非 kg 销售规格。
- 订单表单版本选项、默认解析和显式发布 ID 解析都固定过滤 `factory_supply`；`customer_resale` 不进入 ERP 录单链路。
- `commercial` 订单表单查询读取同一所有者的全部已发布供货快照并按新到旧处理；前端候选以有效 `sku_id`（缺失时 `product_id`）第一次覆盖为准，使派生袋/盒 SKU 分别取最近价格，同时让新版空价格覆盖阻断旧价。顶层 `price_rows` 对同一 SKU 有有效价格时优先于镜像嵌套阶梯，并将显式重量上下界折算为该 SKU 件数范围，避免重复档位或小规格误命中。
- 订单保存优先解析行上精确发布快照；新建行没有精确快照时按 `commercial`、历史 `drip` 的顺序解析挂耳价格。只有前者不存在适用发布时才进入后者；前者存在但缺价则报错。实际命中的 list type 和发布版本写入订单快照。
- 后端发布价格 matcher 与 Vue 录单辅助函数都只返回合法区间命中；前端无匹配时清空自动单价并显示“当前数量无已发布价格，不能保存”。调整到合法数量、补齐价格档或按授权输入大于 0 的手动价后才解除阻断。挂耳盒存在显式盒装档时只匹配盒档，只有完全没有盒档才折算袋数。
- 旧 `drip` 快照只读兼容；新商品价格表仍遵循 PR-415 的通用 Pricing Rule、阶梯模板和 `price_rows` 发布架构。

## TDD 与静态验证

- RED：首轮合同暴露挂耳仍映射到 `drip`、订单表单排除挂耳以及袋/盒价格单位被归一错误；本次补充合同继续暴露同一 SKU 混入历史发布价、`customer_resale` 可被 ERP 解析、数量低于/超出/落在档位空隙时后端与前端仍猜测末档，以及显式盒装档越界后回退袋装档。
- GREEN：`go test ./internal/infrastructure/postgres/orderbeans -count=1`、`go test ./internal/infrastructure/postgres/sales -count=1`、相关 PostgreSQL/HTTP/support 包组合测试和 `go test ./...` 通过；`node --test src/lib/order-entry.test.js` 通过 86/86；`npm run build` 通过（401 modules，只有既有大 chunk 警告）。`node --test src/lib/*.test.js` 为 686/692，仍只有工作区上下文相关的 6 个既有失败，PR-537 目标测试没有新增失败。严格范围合同覆盖熟豆、挂耳袋/盒，发布选择合同覆盖 `factory_supply` 与按派生 SKU `product_id` 的 latest 覆盖。
- 文档合同：`go test ./internal/interfaces/http/support -run TestDev537KMMCommercialOrderContracts -count=1` 固定 PR/DEV/REV 状态、通用价格表优先、历史只读回退、开发环境数据边界、未匹配不猜测和收入/COGS 边界。

## 开发环境数据执行

- 行为代码部署：`./deploy_orderapp.sh development` 已部署 pricing/order 行为提交 `ef72910221c38bf73189346285eb0bc8a9ba96ca`。Docker 内置 `go test ./...`、Vue 401 modules 构建、miniapp typecheck/build、`erp_orderapp` 启动及认证 API 烟测通过；该次上一个应用目录备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260716012948`。最终证据/需求状态提交另行合入并再次部署，其精确 develop 提交、备份和需求 API 烟测记录在交付报告与部署日志中；本行不把行为提交描述为最终部署 tip。
- 开发数据库备份：`/opt/stacks/erp/backups/kferp-dev-kmm-preapply-20260716010537.dump`，11,501,486 bytes，SHA256 `67416924cd041380131ee6992607bcc6b2f59fa457199487894e2229ba3119a8`；已用 `pg_restore -l` 校验可读。
- 输入指纹：`/data/kmm.xlsx` SHA256 `16df2c3d3190bfc2aec0253f9505f18d8f21e0802cd9efb218a390c86a51bebe`；审核映射 SHA256 `fd7009a0c8f34d62b5111e2b441f177f518ba92637f88b9bf112510fe1f2e197`；最终导入器 SHA256 `73613258ac0fbbe0d20493380af9e1ab03e78722fa45d0ce8af2216495eded46`。
- 首次应用报告 `kmm_import_report_apply_1.json`：124 success、40 skipped、1 failed。唯一失败是 PostgreSQL JSONB 把 `1.0` 规范化为 `1` 后触发发布指纹类型误判；发布内容、配置、996 个价格档和 240 个 SKU 项目经语义比较均一致，正式 API 已成功创建发布 `90 / V3.0.18`。报告 SHA256 `6ebae1980d9f71b9b03ec6f6668abacebf99804b5d217219905dd23d4be440bf`，原失败记录保留，没有改写为成功。
- 导入器补充严格 JSON 数值语义规范化后，恢复 dry-run 报告 SHA256 `5e244924a258078719580ddd52d944b3033bf0cefbca0c44252792b2b726e579`，最终 apply 报告 SHA256 `183663c7c622680ec095524c8bbce87334a77d0aa2e0a971050bcf2f4c4eb0b5`；两次均为 2 个 preflight success、107 skipped、0 failed、0 planned writes，没有重复发布。最终部署后的再次 dry-run 报告 SHA256 `572b8867a0e3bc22fd1448b80d519d1c4fc480f8f9aca773ca18744c87cad7a3`，结果仍为 2 success、107 skipped、0 failed、0 planned writes。
- 最终数据：2 个既有物料采购价按 E 列更新，3 个挂耳分摊包材建立/复用；37 个熟豆 BOM 与 19 个挂耳 BOM 均有唯一一致的发布版本并绑定商品；Pricing Rule `12 / KMM供应售价（工作簿阶梯）`、阶梯模板 `12–15` 已启用；官方 `factory_supply` 发布 `90 / V3.0.18` 含 996 个阶梯价格行、240 个 SKU 项目。数据库校验为目标 BOM 56、绑定 56、最新发布版本 56、结构匹配 56，配方差异 0；非目标商品及原有绑定未被改写。
- 真实价格试算：初晓挂耳父商品与派生袋装成本均为 1.34/袋；盒装按 10 袋换算为 13.41/盒，各 BOM 明细同步乘 10；白月光瑰夏 227g 按 0.227 折算。盒装原先保留袋成本的回归先由 RED 测试复现，再修复并通过 `go test ./... -count=1`。
- 待确认并保持现状的 BOM：`ALO TOH#1`、`墨昙`、`果皮茶`、`莓果葡萄`、`醇香拼配`，以及 5 个 KMM 无数值证据的速溶商品。未进入本版 KMM 价格发布的售价：`ALO TOH#1`、`兴福AA`、`墨昙`、`果皮茶`、`森林瑰夏日晒`、`水洗5T`、`莓果葡萄`、`醇香拼配`、`如目达摩 挂耳`。这些记录没有复制相似配方、猜测价格或补 0 元。
- 操作日志：导入窗口记录 POST material create ×3、material update ×2、BOM version create/publish ×56、default BOM bind ×56、Pricing Rule save ×1、tier template save ×4、bean-list publication publish ×1，全部 HTTP 200；通用审计表同步记录对应 create/update/publish/bind 动作。生产环境没有本批写入。

## 保留待验收边界

- 历史 `drip` 兼容的自动化测试已覆盖旧快照类型解析、候选优先级和价格匹配；开发数据库当前没有 `list_type=drip` 的发布，也没有由该回退路径保存的历史订单明细，因此“新录单回退后冻结实际旧发布 ID、版本号与 `list_type=drip`”尚无完整开发环境证据。`ACCEPTANCE_TESTS.md` 对应复合项保持未勾选；本次 KMM 正式 `commercial` 发布、录单和财务验收不依赖该历史路径。

## 录单与财务验收

- 严格边界：SKU-887 `白月光瑰夏 1Kg` 数量 50 超出工作簿最后有限档 24–49kg；空手工价提交返回 HTTP 400 `缺少商品价格表价格`，数据库确认没有生成订单。操作日志保留本次 400 拦截。
- 熟豆：SKU-884 `白月光瑰夏 227g` ×5，命中 `1–6kg`，自动单价 189.77，行金额 948.85；冻结发布 `90 / V3.0.18`、价格单位 227g、库存换算 `227g=0.227kg`、BOM `8337 / V002` 和来源单元格 `供应售价!G29`。
- 挂耳袋：SKU-670 `初晓 挂耳 袋（10g）` ×100，命中 `100–999袋`，自动单价 3.43，行金额 343.00；冻结 `bag / 1袋 / 10g`、发布 `90 / V3.0.18`、BOM `8316 / V002` 和来源单元格 `供应售价!AJ18`。
- 挂耳盒：SKU-671 `初晓 挂耳 盒（10袋）` ×10，命中 `10–99盒`，自动单价 36.30，行金额 363.00；冻结 `box / 10袋/盒 / 10g/袋`、发布 `90 / V3.0.18`、BOM `8316 / V002` 和来源单元格 `供应售价!AK18`。
- 收入：正式 API 创建开发验收订单 `1559 / SO-20260715-0001`，订单合计与未收款金额均为 1654.85；财务月报含税收入 1654.85，drilldown 唯一来源为 `order_revenue / source_id=1559`，与三行冻结成交价完全一致。
- COGS：本次没有创建或完成生产工单，没有发生物料耗用，因此财务 `main_business_cost=0`；未发生生产耗用时记录“COGS 尚未形成”，没有伪造销售即成本。
- 清理：`POST /api/orders/1559/void` 以“`KMM开发环境验收完成`”成功作废订单；订单保留 `is_void=true`，财务收入和 drilldown 回到 0。通用审计表保留 `create` 与 `void`，操作日志保留失败录单 400、成功录单 200 和作废 200。

## 人工验收重点

- 商品价格表中挂耳袋、盒是同一父商品下的两个派生 SKU 价格行，新发布版本类型为 `commercial`。
- ERP 录单不填手工价即可命中对应袋/盒阶梯，价格单位不被改成 kg 或 lb，价格来源显示实际发布版本。
- 同一所有者部分发布时，分别核对袋/盒派生 SKU 的有效 `sku_id`（缺失时 `product_id`）：每个 SKU 使用最近包含自身的 `factory_supply` 快照，新版空价 SKU 不回退旧价，`customer_resale` 版本不出现在录单且不能显式保存。
- 熟豆、挂耳袋和挂耳盒分别测试低于起订量、高于有限最高量和档位空隙，确认自动价为空、页面显示“当前数量无已发布价格，不能保存”并阻断保存；有显式盒装档时盒数越界不回退袋装档，只有没有盒装档时才折算袋数。
- 待人工确认数据没有进入 BOM 或已发布价格表；开发环境以外没有本批写入。
- 订单收入正确；未发生生产耗用时界面或验收记录没有虚构 COGS。
