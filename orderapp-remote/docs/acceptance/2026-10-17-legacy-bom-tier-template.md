# PR-696 旧价格表模板兼容

## 范围与依据

- 分支 codex/legacy-bom-tier-template-20261017，起点 origin/develop@a3fe77aa。
- 开发数据只读诊断：初晓-商品来源227g的2–13件档关联模板18，旧发布没有完整快照；旧表部分参数与现模板吻合，模板更新时间早于发布。
- 不修改共享开发业务数据，不自动重新发布、回填旧价格表；不增加计价方式或数据库迁移。
- 新表继续使用发布快照。旧表明确关联模板且时间依据、已有部分参数一致时直接计算；否则员工确认当前模板的上下文版本指纹。模板行加共享锁保证核对、报价和订单提交期间一致。
- 订单保存完整实际模板、模板更新时间、采用依据、目标计算结果、单价和档位；原订单数量编辑只读服务器冻结依据，复制/重新计价重新判定。

## RED

- /tmp/kferp-legacy-backend-red.log：真实隔离PostgreSQL HTTP期望旧表报价200，实际400「缺少完整的已发布计算模板…」。
- /tmp/kferp-legacy-vue-red.log、/tmp/kferp-legacy-mini-red.log：两端页面函数无法保存服务器确认版本，新增用例失败，已有用例通过。

## GREEN

- /tmp/kferp-legacy-backend-green.log：ERP/员工预览、确认、保存、冻结数量编辑、复制旧确认失败、伪造确认、模板停用、缺少关联、原完整快照优先、目标成本、价格表内容和数量不变、操作日志及换算组接口回归通过。
- /tmp/kferp-legacy-vue-green.log、/tmp/kferp-legacy-mini-green.log：两端确认、重报价、规格清理、迟到错误隔离通过。
- 全量 Go、Vue 1377 tests/Vite build、小程序339 tests/vue-tsc/development build通过；新增复制确认清理用例后定向复核。日志 /tmp/kferp-legacy-all.log、/tmp/kferp-legacy-mini-all.log、/tmp/kferp-legacy-typecheck.log、/tmp/kferp-legacy-mini-build.log。
- 本地真实Vue组件+模拟API，482px宽：换规格清档、模板变化提示、点击确认后展示「已确认当前模板」和单价，保存按钮恢复可用。截图 ERP-current-template-narrow.png；未提交共享业务订单。
- 代码自审：模板关联来自服务器已发布行；锁与版本指纹涵盖客户/商品/表/档/双方规格；已有快照优先；复制和主动重算清旧确认；服务器保存复核，历史客户端快照不可信。
- 开发部署前检查服务器仅约1GB可用，确认无构建后执行 go clean -cache，仅删除可再生成的Go编译缓存，恢复到4.6GB；业务数据/卷/源码备份/回滚镜像保留。开发交付待补录。
- 手册：OP_MANUAL_BOM_ORDER_PRICING.md，及订单/成本/员工小程序/换算组说明；现有录单帮助打开同一手册。

## 交付边界

仅开发环境与开发版小程序包。生产发布、微信上传审核发布、真实业务/真机验收未执行。

## 开发交付回执（2026-10-18）

- GitHub PR #208 已合并：https://github.com/Backstage2539/KFerp/pull/208。实现提交 8a54df883f43a23d6b99e99861c3ae780949e47e。
- 已部署 develop@a892ab5e6b6d911a0f2d81d205b59875394c85a6。命令：KFERP_MINIAPP_DEVELOPMENT_EXPORT_DIR=/Users/yiiiple-work/KFerp-legacy-tier-delivery-20261017/KFerp-miniapp-mp-weixin-dev ./deploy_orderapp.sh development，退出0。
- 服务器 Vue 1378 tests/build、小程序340 tests/typecheck/build、完整Go和镜像内Go测试通过。
- 运行：erp_orderapp running，erp_postgres healthy；login200、匿名quote401、认证手册包含旧表兼容/确认内容、PR-696为review。运行包RELEASE_INFO与提交/开发环境一致。
- 源码回滚：/opt/stacks/erp/orderapp.backup.deploy-20261018153938-a892ab5e6b6d；回滚镜像：kferp-orderapp-rollback:development-20261018153938-a892ab5e6b6d。
- 开发包：/Users/yiiiple-work/KFerp-legacy-tier-delivery-20261017/KFerp-miniapp-mp-weixin-dev；ZIP：KFerp-miniapp-dev-a892ab5e.zip。
- 包校验：24页面、96页面文件，development API、当前模板确认控件及版本字段存在，无project.private.config.json，ZIP完整性通过。
- SHA256：aea71725cfa5fecba0382d2453cb522eb405660aef5965ba16180b96565e38e6。
- 部署前只清理可再生成的Go编译缓存；部署后约3.0GB可用。未删除业务数据、数据库卷、源码备份或回滚镜像。
- 日志、截图、ZIP和PACKAGE_CHECK.txt存放上述交付目录。仅本地模拟数据界面检查与开发只读运行检查；生产部署、微信上传审核发布、真实业务/真机验收未执行。
- 本回执为部署后文档补录，不改变运行版本。
