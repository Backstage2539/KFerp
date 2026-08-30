# PR-615 价格试算提速、展示统一与模板快捷编辑验收记录

## 范围

- 批量价格试算按商品范围加载，并去重生产选项与成本上下文。
- 价格表预览、PDF 和公开页使用同一整数/两位小数展示规则。
- 平铺价格行新增复用式价格模板编辑抽屉，保存后定向重算当前草稿。
- 只交付 development；不操作 `main`、production 或生产业务数据。

## 自动验证

- RED：范围加载接口缺失；同商品四档重复加载生产选项和成本明细；价格 `38.2` 被取整为 `38`；模板编辑按钮、共享表单和定向缓存失效缺失。
- GREEN：见 `internal/application/costing/service_test.go`、`internal/infrastructure/postgres/costing/repository_test.go`、`internal/interfaces/http/costing/costing_api_test.go`、`frontend-vue-shell/src/lib/bean-list-pdf.test.js`、`costing-price-list-workflow.test.js` 和 `costing-bean-list-version-ui.test.js`。
- GREEN：`scripts/verify_kferp.sh all` 通过；Go 全量、Vue 1043/1043 和 Vite 6596 modules 构建均通过，`git diff --check` 通过。
- 逻辑加载次数：同一商品四档从“全量商品输入 + 4 次生产选项 + 4 个重复成本明细输入”收敛为“1 次商品范围输入 + 1 次生产选项 + 1 个唯一成本明细输入”；结果仍按原请求顺序返回。
- 远端预检：功能提交 `f16f67c0f5cbe45c9a544a2687100c258fb3a70d` 在 development 构建设置下通过 Vue 1043/1043、Vite 6596 modules、小程序 219/219、类型检查、Go 全量和隔离镜像构建，未改服务器源码或容器。
- 合并与部署：`develop@b437c743439e2062062d7fcba227ec33834b9c33` 已部署 development；源码备份 `/opt/stacks/erp/orderapp.backup.deploy-20260829001656-b437c743439e`，回滚镜像 `kferp-orderapp-rollback:development-20260829001656-b437c743439e`。
- 健康检查：`erp_orderapp` running / restart 0，PostgreSQL healthy，登录页 HTTP 200，未认证受保护 API HTTP 401，认证需求与成本 API HTTP 200。
- 真实四档：开发现有 BOM 规格使用真实四档模板首次调用 `POST /api/costing/pricing-rule-trials` 为 `0.134899s`，HTTP 200、4/4 行无错误且全部生成正价格，响应顺序为 `[0,1,2,3]`；没有保存或发布价格表。
- `main`、production、生产业务数据和固定小程序开发包均未改动。

## 人工验收边界

- Van 在 development 商品价格表选择一个包含四个档位的商品，首次生成后确认两秒内出现全部价格。
- 编辑当前引用模板并保存，确认仅相关自动行进入“价格计算中”，人工价保留，重算完成后可撤销人工修改到新基准。
- 确认预览、生成 PDF 与公开页的小数展示一致。
