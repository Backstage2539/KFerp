# PR-431-CUSTOMER-RESALE-BEAN-LIST Acceptance

## Scope
- 小程序客户可基于可见的工厂供货豆单生成自己的客户销售豆单。
- 客户销售豆单保存为 `publication_purpose=customer_resale`，只用于 PDF/长图分享，不参与向工厂下单、ERP 录单、结算、费用中心或履约计价。
- 工厂供货豆单继续使用 `publication_purpose=factory_supply`，现有记录默认回填为该用途。

## Implemented
- `bean_list_publications` 增加用途字段，ERP 产品价格表版本列表增加“工厂供货豆单 / 客户转售豆单”用途筛选。
- 阶梯价模板增加“允许客户转售豆单使用”开关；小程序只返回 active 且授权的模板。
- 小程序新增客户转售豆单接口：列表、编辑器、草稿保存、发布、PDF、PNG。
- 客户转售发布按来源工厂供货快照、授权模板档位、统一加价、倍率加价和单品覆盖生成最终展示价格快照。
- 小程序“我的豆单”页增加轻量编辑器，支持来源豆单、模板、商品选择、品牌/说明、背景/样式、标签、草稿、发布和 PDF/长图分享。
- PDF 输出复用现有豆单 PDF 逻辑，PNG 使用服务端长图缓存。

## Evidence
- RED tests were added before implementation for service calculation, Mini API, repository filtering, PNG renderer, costing purpose filtering, ERP Vue purpose filter, gradient template authorization and miniapp editor helpers/UI anchors.
- GREEN targeted backend:
  - `go test ./internal/application/customerportal -run 'TestPublishResaleBeanList' -count=1`
  - `go test ./internal/interfaces/http/customerportal -run 'TestMiniResaleBeanList|TestMiniBeanListPDF' -count=1`
  - `go test ./internal/interfaces/http/costing -run 'TestBeanListPublicationAPISupportsPurposeFilter|TestBeanListPublicationAPI' -count=1`
  - `go test ./internal/infrastructure/postgres/customerportal -run TestResaleBeanListPageSeparatesFactorySupplySnapshotsAndAuthorizedTemplates -count=1`
  - `go test ./internal/infrastructure/pdf -run TestBeanListRendererRenderPNGProducesLongShareImage -count=1`
- GREEN targeted frontend/miniapp:
  - `node --test src/lib/gradient-templates.test.js src/lib/costing-bean-list-version-ui.test.js`
  - `npm test -- src/utils/mainTabs.test.ts src/utils/resaleBeanList.test.ts`
  - `npm run typecheck` in `miniapp`
- GREEN broader/build:
  - `go test ./internal/application/customerportal ./internal/interfaces/http/customerportal ./internal/interfaces/http/costing ./internal/infrastructure/postgres/customerportal ./internal/infrastructure/pdf ./internal/interfaces/http/support ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing -count=1`
  - `npm run build` in `orderapp-remote/frontend-vue-shell`
  - `scripts/verify_kferp.sh frontend-build`
  - `npm run build:mp-weixin` in `miniapp`
- Visual artifact check:
  - PDF: `../customer-resale-bean-list-artifacts/customer-resale-bean-list.pdf`
  - PDF screenshot: `../customer-resale-bean-list-artifacts/customer-resale-bean-list.pdf.png`
  - Long PNG: `../customer-resale-bean-list-artifacts/customer-resale-bean-list.png`
  - Quick Look PDF screenshot and long PNG show version, brand/intro, recommendation tag, price and changelog without overlap.
- Integration/deploy:
  - Feature branch `codex/customer-resale-bean-list-20260606` pushed to origin.
  - Merged into `develop` via `1eb60a84`; development stack later advanced to `b3306afb` and still contains PR-431 source markers.
  - Smoke checks passed: `/app/` returns 303 to `/app/orders`, `/app/vue-shell/` returns 200 with BasicAuth, unauthenticated mini resale list/PDF/PNG endpoints return 401, and `erp_orderapp` is up.

## Manual Updates
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/customer-portal-miniapp-test.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

## Pending Product Acceptance
- Perform live ERP/miniapp acceptance with a real customer binding and mini token on 产品价格表、阶梯价模板、小程序“我的豆单”和操作日志.
