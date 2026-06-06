# PR-433 小程序我的商品与客户商品价格表验收记录

## 范围

- 首页不放 `我的商品`，个人中心进入 `我的商品`。
- `我的商品` 展示商品分类、商品价格表、已发布商品价格表和我的价格表设置。
- 分类编辑复用 ERP 商品分类模板体系，客户首次编辑会派生客户专属模板。
- 商品价格表按 `list_type/list_type_label` 分组；客户发布商品价格表按类型折叠。
- 编辑器去掉覆盖档位和单品价，保留统一加价、倍率加价、标签、说明、预设色板和 1/2/3 卡片数。
- 小程序构建：部署脚本必须构建小程序 `mp-weixin`。

## 本地验证

- [x] Go/API：客户商品接口、分类写接口、旧单品覆盖忽略、授权模板过滤。证据：`go test ./...`。
- [x] Miniapp：入口、分类、分组、折叠、编辑器和输出预览。证据：`npm test`。
- [x] Build：`npm run typecheck`、`npm run build:mp-weixin`、Vue shell `npm run build`。
- [x] Deploy smoke：远端 `miniapp/dist/build/mp-weixin` 存在；部署文档含 PR-433；`/app/vue-shell/` 与 `/app/api/req/product?limit=500` 返回 200；小程序接口无 mini token 返回 401。
