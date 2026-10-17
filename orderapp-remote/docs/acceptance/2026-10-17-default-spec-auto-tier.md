# PR-695 默认规格自动匹配档位

日期：2026-10-17。DEV-778/779/780。产品验收：待 Van。

## 实现与兼容

- 目录接口新增已发布数量区间和计价口径；真实规格 ID 匹配，不解析规格名或档位标签。无数据库迁移。
- 默认规格按当前件数唯一匹配，包含上下限与无上限档位；数量变化更新档位。未直接定价、未命中、重叠或缺失依据时说明原因并要求手选，仍无匹配时保留有效手动选择。
- 非默认规格每次切换清空旧档，之后数量变化保持人工档位；切回默认恢复自动。所有报价仍由现有服务器服务计算。
- 已保存订单数量编辑冻结原价；主动重新计价重新读取当前目录。复制和草稿恢复刷新目录与报价；非默认规格只保留有效的原手动档位。
- 数量、来源档位、客户、商品、价格表及目标身份纳入迟到响应隔离。配置加载期间避免普通价格覆盖。
- 不变：换算组、客户自行下单范围、服务器复核、订单操作日志、目标生产库存身份和价格表内容。

## RED

- Vue：新增 resolveBOMTier 导出缺失，node:test 失败；`/tmp/kferp-auto-tier-vue-red.log`。
- 小程序：新自动匹配函数缺失，换规格仍保留旧档，5失败/2通过；`/tmp/kferp-auto-tier-mini-red.log`。
- Go：API 元数据断言在 MinQty/MaxQty/QuantityBasis 不存在时编译失败；`/tmp/kferp-auto-tier-go-red.log`。

## GREEN

- `scripts/verify_kferp.sh backend`：完整 Go 通过（`/tmp/kferp-auto-tier-final.log`）；`scripts/verify_kferp.sh frontend`：Vue 1375 tests、Vite build通过（`/tmp/kferp-auto-tier-frontend-final.log`）。原草稿恢复静态断言按新“保留规格后重新报价”流程更新。
- `ORDERAPP_TEST_DATABASE_URL='postgres:///postgres?host=/tmp' go test ./internal/appmain ./internal/interfaces/http/sales -run 'TestBOMOrderPricingPostgresHTTP|TestConversionPostgresHTTPAndFrozenOrder' -count=1`：隔离 PostgreSQL ERP/员工接口通过；`/tmp/kferp-auto-tier-pg.log`。
- API 断言已发布 min/max/basis（含0下限、47上限和开放上限），原创建/编辑/复制/冻结/重新计价/伪造金额/跨客户商品/库存目标身份/价格表不变/操作日志及换算组测试通过。
- Vue 和小程序页面函数行为测试覆盖自动跨档、每次换规格清档、回默认恢复、手动回退、同档待报价时修改数量、过期响应和冻结后主动重算。
- 小程序55文件337 tests、vue-tsc、development build：`/tmp/kferp-auto-tier-mini-final.log`、`/tmp/kferp-auto-tier-typecheck-final.log`、`/tmp/kferp-auto-tier-mini-build-final.log`。
- 本地真实 OrderEntryView + 模拟 API：默认227g，2袋48元→14袋40元；切500g清档并禁用保存；选来源档后数量100保留96元。482px iframe验证文案和控件可用。仅本地界面验证，不是共享开发数据业务验收或小程序真机验收。
- 代码复核：原数字型阶梯下限兼容小数元数据，自动按件匹配只接受有效整数区间；默认档不会从其他同名规格推断；主动重算刷新目录；保存仍依赖服务器复核。
- 手册：OP_MANUAL_BOM_ORDER_PRICING.md及订单/成本/小程序/换算组引用段；Vue录单帮助和单价状态提示同步。

## 开发交付

功能分支 `codex/default-spec-auto-tier-20261017`，基于 `origin/develop@f9ec1da6`。
仅 development 部署及开发版包；生产、微信上传审核发布、真实业务验收分别处理。
最终提交、部署备份、健康检查和开发包校验将在部署完成后补录。
