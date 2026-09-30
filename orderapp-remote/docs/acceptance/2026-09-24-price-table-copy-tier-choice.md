# PR-671 客户价格表完整复制、员工手动选档与商品展示修正

日期：2026-09-24。功能分支：`codex/price-table-copy-tier-choice-20260924`。基线：`origin/develop@8f241a9b2969fc4bf930b573b98d54ebff9a791f`。验收人：Van。

## 交付范围

- 客户当前编辑价格表可从已发布公共表或有权限查看的客户供货表完整复制；服务端按当前客户已存在的商品档案/有效规格身份取交集，保留所有最终档价和配置，将当前命名表覆盖保存为草稿。无交集不写入，不建立来源同步；复制写入操作日志并以请求标识支持重试。
- ERP 与小程序员工录单默认按数量匹配发布档位；员工可跨数量门槛手选当前商品/规格的发布档价，数量变化后保持手选，恢复自动后重新匹配。员工直接改单价仍为独立方式；客户自助下单不提供或接受手选档。
- 商品配置抽屉按商品真实档案归属显示；行业字段按已冻结的模板身份分行、同模板字段合并展示，适用于预览、分享和打印/PDF，历史无模板快照保持原有分行语义。

## TDD、接口与自动验证

- RED：新增普通无 SKU/BOM 规格商品身份回归 `TestFilterBeanListPublicationCopyMatchesProductWithoutSKUOrBOMSpec` 首次运行失败，暴露商品行被误判为 SKU 身份、客户交集错误为空；GREEN 后普通商品以商品档案身份命中。规格身份、只复制 A/B/C 来源中的客户 A、各档人工最终价、跳过计数和价格冻结另有应用层回归覆盖。
- API：`TestPublicationCopyAPIUsesScopedSourceAndServerOwnedCustomerTarget` 覆盖来源作用域、目标客户归属从服务端上下文派生及 preview/copy 路由；`TestPublicationCopyAPIRejectsUnauthorizedSourcesAndActors` 覆盖客户转售来源、缺失幂等请求标识及未授权角色。发布复制服务测试覆盖目标表完整替换、保留客户商品名和表名、档位最终价冻结、其他命名表内容不变、无交集不保存和重复请求读取原草稿。
- Go 全量：`./scripts/verify_kferp.sh all` 通过（包括 `go test ./...`、Vue 全部 Node 测试、Vite 构建、diff 与冲突标记检查）。
- Vue 全量：`node --test`，1,233/1,233 断言通过；`npm run build` 成功。
- 小程序：`npm test`，42 个测试文件、264/264 用例通过；`npm run typecheck` 成功；`npm run build:mp-weixin:development` 成功。编译产物仅用于开发包验证，未上传微信平台。

## 操作手册与进度

- 已更新 `docs/OP_MANUAL_COSTING.md`、`docs/OP_MANUAL_ORDER_SALES.md`、`docs/OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`、`docs/OP_MANUAL_CUSTOMER_PORTAL.md`、`docs/OP_MANUAL_INVENTORY_MATERIALS.md` 及 `docs/OPERATION_MANUALS.md`；ERP Vue/Vite 手册入口继续读取单一来源 `docs/`。
- PR/DEV 跟踪见 `internal/interfaces/http/support/req_store.go` 与仓库根 `ACTIVE_REQUIREMENTS.md`。DEV-671-01 至 DEV-671-05 的实现和自动验证完成；DEV-671-06 等开发环境发布和验收记录。

## 集成、开发环境部署与只读验收

- 功能 PR #141 已合并。功能提交：`e6bec3ba7e2f9ba777bf985db48347ea1d9074bc`；功能合并提交：`b4f5ad6b1693480c5c7735eb2d4e528a3f253a9f`。交付状态 PR #142 已合并并部署；当前 development 应用运行提交：`f50d51e492a5c2587cf04f84910d5e8ec1e319d1`。随后 PR #143 和 #144 仅更新部署证据文档，没有改运行时代码，因此无需再次部署。
- 按 `./deploy_orderapp.sh --preflight development` 完成远端预检（Vue shell 测试/构建、小程序 42 个文件共 264 项测试/类型检查/开发构建、Go 全包测试及隔离 Docker 镜像构建）；无服务栈或容器变更。随后以 `./deploy_orderapp.sh development` 部署，重建并仅重启 `erp_orderapp`。
- 功能版本先于 2026-09-24 03:03 部署为 `b4f5ad6b1693480c5c7735eb2d4e528a3f253a9f`。为同步 PR/DEV 看板的 `review`/`done` 状态，PR #142 更新了启动种子与交付证据，并于 03:23 再次部署；当前服务运行 `f50d51e492a5c2587cf04f84910d5e8ec1e319d1`。第二次部署再次通过 Vue shell 测试/构建、小程序 42 个文件共 264 项测试/类型检查/开发构建、Go 全包测试及 Docker 镜像构建；只重启 `erp_orderapp`。
- 最终部署服务器旧源码备份：`/opt/stacks/erp/orderapp.backup.deploy-20260924031554-f50d51e492a5`；回滚镜像：`kferp-orderapp-rollback:development-20260924031554-f50d51e492a5`。部署后 `erp_orderapp`、`erp_docconvert` running，PostgreSQL healthy；开发登录页 `https://dev.qacoohee.com/app/login` 返回 HTTP 200；未登录访问 `/app/` 返回 303 登录跳转；带 Basic Auth 的 `/app/vue-shell`、`/app/api/req/product`、`/app/api/req/dev` 均返回 HTTP 200。看板读取到 PR-671=`review`、DEV-671-01 至 DEV-671-06=`done`。
- 开发小程序产物的 72 个声明文件清单校验通过；最终产物同步至 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`，旧目录保留于 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev.backup-20260924032319-f50d51e492a5`。此操作仅准备本地开发包，没有上传微信平台。
- 本次未对共享开发业务数据执行价格表复制或创建/保存订单，因此此处的真实业务工作流验收留给 Van 使用隔离/指定测试客户完成；Van 业务验收待进行。
- 生产部署、生产写入、微信小程序上传/审核/发布均不在本次授权范围。
