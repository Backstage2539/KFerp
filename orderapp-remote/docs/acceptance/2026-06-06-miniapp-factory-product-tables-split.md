# 小程序工厂商品表与我的商品编辑拆分验收

Requirement: PR-434-MINIAPP-FACTORY-PRODUCT-TABLES-SPLIT

## Scope
- 个人中心新增 `工厂商品表` 和 `我的商品` 两个入口。
- `工厂商品表` 只读展示当前客户可见的最新工厂商品价格表，支持大类/小类折叠和 `PDF`、`长图` 输出。
- `我的商品` 聚焦客户自己发布的商品价格表版本，`价格表设置` 独立成页。
- 价格表设置支持从 `工厂价格表` 或 `我的已发布价格表` 复制；复制客户旧版本时保留展示配置，但价格追溯仍使用原工厂价格来源。

## Local Evidence
- [x] RED: `go test ./internal/application/customerportal -run TestPublishResaleBeanListCopiedFromCustomerVersionKeepsFactoryPriceSource -count=1` initially failed on missing `CopySourceType` / `PriceSource`.
- [x] RED: `go test ./internal/interfaces/http/customerportal -run TestMiniBeanListPNGAPIReturnsFactoryLongImage -count=1` initially failed with invalid PNG request.
- [x] RED: `go test ./internal/interfaces/http/support -run TestDev434MiniappFactoryProductTablesSplit -count=1` initially failed because PR-434 seeds and page files were absent.
- [x] RED: `npm test -- src/utils/mainTabs.test.ts src/api/customerPortal.test.ts` initially failed because new pages and factory PDF/PNG helpers were absent.
- [x] Miniapp unit: `npm test -- src/utils/mainTabs.test.ts src/api/customerPortal.test.ts`.

## Acceptance Checklist
- [ ] 个人中心显示 `工厂商品表` 和 `我的商品`，首页不出现 `我的商品` 主入口。
- [ ] 工厂商品表按 `list_type/list_type_label` 展示最新 factory_supply 价格表，并按快照分类收起展开。
- [ ] 工厂商品表和客户已发布版本的 `PDF`、`长图` 按钮可打开输出。
- [ ] 我的商品首屏只显示摘要、已发布商品价格表折叠版本和 `价格表设置` 入口。
- [ ] 设置页可从工厂价格表或客户已发布版本复制。
- [ ] 复制客户旧版本发布时，价格来源仍追溯原 factory_supply，不重复倍率或统一加价。
- [ ] 复制草稿可新增、删除、改名和收起分类；商品右侧 `商品配置` 可设置 `上新`、`推荐` 和标红词。
- [ ] 页面不出现 `预览 PDF`、`预览长图`、`覆盖档位`、`单品价`。
- [ ] 部署后远端存在 `miniapp/dist/build/mp-weixin`，并通过 `/app/`、Vue shell 和小程序接口鉴权烟测。
