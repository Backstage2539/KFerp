# PR-677 商品创建器 V6 验收记录（2026-10-02）

## 范围

V6 只提供物料、商品、工艺和 BOM 组装；物料档案可预设多行并使用顺序命名组合，运行时可选现有业务分类和填写暂估采购价。正式采购及收货仍由原业务页面完成，实际收货价（含 0）清除暂估价。旧 V1–V5 模板和运行维持原语义。

## 自动验证

- `scripts/verify_kferp.sh all`：通过。包含 Go 全量测试、Vue/Vite 全量 Node 测试、前端生产构建和变更格式检查。
- 商品创建器定向前端测试：38 项通过，覆盖 V6 模块目录、购入模块隐藏、物料行预设、变量命名和分类/工艺文案。
- 商品创建器应用及 HTTP API 定向测试：通过。V6 模块目录返回 4 个 V6 模块且不暴露物料购入；历史版本模块仍保留。
- Vite 构建通过；保留仓库现有 PDF 预览大分块体积提示。
- `git diff --check`：通过。

## 待开发环境验证

- 本机未配置 `ORDERAPP_TEST_DATABASE_URL`，因此未执行独立 PostgreSQL 数据库集成测试。
- 已合入 `develop`，合并提交 `79a17628ddd665c7bdb833595d07105827f6b20b`；development 部署于 2026-10-02，备份源码 `/opt/stacks/erp/orderapp.backup.deploy-20261002003406-79a17628ddd6`，回滚镜像 `kferp-orderapp-rollback:development-20261002003406-79a17628ddd6`。
- 部署脚本全量 Go/Vue/小程序检查通过；远端 `erp_orderapp`、`erp_postgres`、`erp_docconvert` 正常运行，开发登录页 HTTP 200。按本次范围未同步微信小程序开发版/体验版产物。
- 仍需在 development 用已登录业务账号补做认证 API 和浏览器页面烟测；在 666/768/1024/1440 宽度检查分类抽屉、运行表单和名称组合。
- 仍需在 development 创建 V6 验收模板并走通一次新商品创建，核对物料/商品/BOM 分类、默认规格、工艺和暂估成本。本次会话没有可用的已登录浏览器交互能力，因此没有代建业务档案。
- Van 的业务验收仍待确认；本次不触碰生产环境或生产档案。

## 手册

- [商品创建器操作手册](../OP_MANUAL_PRODUCT_CREATOR.md)
- [操作手册索引](../OPERATION_MANUALS.md)
