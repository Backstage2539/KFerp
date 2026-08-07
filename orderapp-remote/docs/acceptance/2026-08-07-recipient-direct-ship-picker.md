# PR-583 连写收货地址解析与一件代发商品选择验收

## 范围

- 修复共享收货信息解析器对“完整行政区地址 + 详细地址 + 紧邻姓名 + 手机号”无分隔文本的解析，并保护低置信度地址不被清空。
- 员工录单与一件代发共用商品族搜索/分类底部弹层；一件代发商品、规格、数量常驻展示并支持多行。
- 不新增接口、数据库字段、迁移或操作日志类型；发货提交、取消、库存隔离、FIFO、跨仓和幂等链路保持不变。

## RED 证据

- `go test ./internal/application/customer -run TestParseRecipientTextMatchesERPAddressParserExamples -count=1`：精确样例、连写两字姓名、复姓和纯地址保护均失败；旧实现把手机号前整段当作姓名，并把地址清空。
- `go test ./internal/interfaces/http/customerportal -run TestRecipientParseAPIUsesOneContractForERPAndMiniEmployeeTokens -count=1`：ERP、员工小程序和客户代发三种会话返回同一错误结果。
- `npm test -- --run src/utils/directShipDraft.test.ts src/utils/productFamilyPickerSheet.test.ts src/utils/employeeOrderPage.test.ts src/utils/customerClosedLoopPages.test.ts`：新共享弹层、初始空行、三控件常驻和草稿行合同在实现前失败；旧一件代发常驻展开搜索、分类、商品和规格按钮，选择某个规格后才出现数量行。
- 独立复审补 RED：带空格行政区地址漏拆、手机号后明确姓名被启发式覆盖、尾随备注覆盖紧邻姓名、稀有姓氏兼容，以及只改空行数量仍被静默忽略，均先以定向失败用例复现后修复。

## GREEN 证据

- `go test ./internal/application/customer ./internal/interfaces/http/customerportal -count=1` 通过；精确样例解析为王心星、云南省、普洱市、景谷傣族彝族自治县及不含姓名的详细地址。“万达广场 + 手机号”等低置信度尾部保留完整地址且姓名为空。
- `scripts/verify_kferp.sh backend` 全量 Go 测试通过；`go vet ./internal/application/customer ./internal/interfaces/http/customerportal` 通过；PR-583 需求/手册支持合同和更新后的 PR-566 共享选择器合同通过。
- miniapp 定向测试通过；全量 `npm test` 为 31 个测试文件、195 项通过；`npm run typecheck` 通过；`npm run build:mp-weixin:development` 完成 development 构建。共享弹层真实打开、分类、搜索、关闭和重开重置留到 DevTools 验收。
- `TestMiniDirectShipCatalogExcludesPublicOtherCustomerFrozenAndReservedStock` 通过：同 SKU 的公共仓、其他客户仓、冻结批次均未进入当前客户目录，已有预留从可用量扣除。既有 closed-loop 测试继续覆盖 FIFO、跨仓、权威库存重验、幂等、取消释放和审计。
- 开发 ERP 部署提交、服务器备份、容器/HTTP/日志检查和开发小程序固定目录备份待部署后补充。

## 手工验收

- [ ] 在一件代发粘贴现场连写收货信息，核对收件人、电话、省、市、区县和详细地址正确，仍可手工修改。
- [ ] 点击初始商品框，确认底部弹层同时具备搜索、分类和商品列表；分别用商品名称、拼音/首字母、编码和 SKU 搜索。
- [ ] 选择多规格商品，确认自动带默认规格且规格框可改选；数量初始为 1，切换商品后数量保留、旧规格不残留。
- [ ] 新增多行、删除到最后一行、保留空附加行、制造半填行和非正数量，核对页面校验与请求内容。
- [ ] 用当前客户成品仓商品完成发货预览，确认其他客户/公共仓商品不出现；不自动提交真实发货。

## 交付状态

- 功能分支：`codex/pr583-recipient-directship-20260807`。
- 产品需求：`review`；`REV-583-RECIPIENT-COMPACT-ADDRESS-DIRECT-SHIP-PICKER` 等待 Van 在 development 验收。
- `main`、production、微信上传/审核/正式发布：不执行。
