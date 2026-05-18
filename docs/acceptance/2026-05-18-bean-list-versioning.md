# 豆单版本管理验收记录

日期：2026-05-18

## 需求
- 豆单发布后进入版本列表，同一版本 PDF 生成一次后缓存复用。
- 录单选择客户时记录豆单版本；客户有专属豆单时可选版本，默认最新；无专属豆单时使用公共豆单。
- 小程序默认展示最新豆单，并在客户首次按新版下单前提示更新摘要；确认后不重复提示。
- 客户门户配置可为有专属豆单的客户选择展示最新或固定版本。

## 验证
- `go test ./internal/application/customerportal ./internal/interfaces/http/customerportal ./internal/infrastructure/postgres/customerportal -run 'TestBeanListDiffDetectsAddedRemovedAndChangedItems|TestMiniBeanListPDFAPIReturnsPDFDownload|TestLoadBeanListServicePageUsesFixedCustomerPublication' -count=1`
- `go test ./internal/application/sales ./internal/interfaces/http/sales -run 'TestSaveOrderCommandUsesTypedFields|TestOrderAPISavesSelectedBeanListPublicationVersion' -count=1`
- `go test ./internal/infrastructure/postgres/sales -run 'TestOrderQueries|TestSalesSchema|Test.*BeanList' -count=1`
- `node --test src/lib/order-entry.test.js`
- `npm test -- --run src/api/customerPortal.test.ts`

## 备注
- 本地 API 订单持久化测试在没有数据库连接时会按既有测试约定跳过；其余后端包编译和目标用例通过。
