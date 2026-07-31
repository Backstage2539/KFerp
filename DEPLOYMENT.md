# DEPLOYMENT - KFerp 部署流程与教程

> 目标：把"从本地代码到线上可访问"这条链路写成可重复执行的步骤。
> 
> 当前线上环境：
> - 生产前端 URL：`https://erp.qacoohee.com/app/`
> - 开发前端 URL：`https://dev.erp.qacoohee.com/app/`
> - 服务器：`root@1.12.242.58`
> - 生产部署目录：`/opt/stacks/erp-production`
> - 开发部署目录：`/opt/stacks/erp`

---

## 新前端页面

### React BOM 配方维护页面
- 访问路径：`/app/bom-react`
- 技术栈：React 18 + TypeScript + Vite + TanStack Query + Tailwind CSS
- 源码：`orderapp-remote/frontend/`

本地开发：
```bash
cd orderapp-remote/frontend
npm install
npm run dev  # http://localhost:3000
```

---

## 0. 前置条件

### 0.1 你需要有的东西
- 服务器 SSH 权限（能登录 `root@1.12.242.58`）
- SSH agent/default key 可直接登录；如需指定私钥，设置 `KFERP_SSH_KEY=/绝对路径/私钥`
- 本地有基础工具：
  - `git` / `ssh` / `tar`
  - 本地不需要 Node、Go 或 Docker，也不会执行 npm、Go、Vue/UniApp 或 Docker 重型构建

### 0.2 服务器端依赖
服务器 `/opt/stacks/erp-production` 和 `/opt/stacks/erp` 分别使用 `docker compose` 管理服务，需要：
- Docker Engine
- Docker Compose v2（`docker compose` 子命令）

---

## 1. 线上结构说明（你在服务器上会看到什么）

登录服务器：
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58
```

核心目录：
- 生产：`/opt/stacks/erp-production`
- 开发：`/opt/stacks/erp`
- 各环境目录下：
  - `docker-compose.yml`：compose 定义
  - `.env`：环境变量（包含数据库密码、ORDERAPP_PASS、WECHAT_MINI_APP_ID、WECHAT_MINI_APP_SECRET 等）
  - `orderapp/`：orderapp 源码（会被脚本覆盖同步）
  - `orderapp_data/`：orderapp 持久化数据（assets 等）
  - `postgres_data/`：Postgres 持久化数据

---

## 2. 一键部署（推荐）

仓库根目录有脚本：`deploy_orderapp.sh`

### 2.1 脚本做了什么
1. 本地只做轻量守卫：工作区必须 clean。预检要求功能分支已推送且 HEAD 等于其 `origin` upstream；正式部署要求分支匹配环境，并且提交与对应的 `origin/develop` 或 `origin/main` 完全一致。
2. 用 `git archive` 把已提交源码传入服务器唯一的 `/tmp/kferp-orderapp-release-*` 目录；不会上传本地 `node_modules`、构建缓存、未跟踪文件或密钥。
3. 服务器取得统一发布锁后串行执行 Vue 依赖安装/构建、小程序依赖安装/单测/类型检查/构建、`go test -p 1 ./...` 和 Docker 构建。
4. Node 内存上限默认为 768 MB，npm、Go、Compose 都限制并发，避免 8 GB Mac 本机和服务器同时抢占大量资源。
5. 所有步骤通过后才提升服务器源码，备份上一版源码和镜像，并只重启 `orderapp`。提升后失败会自动恢复上一版源码和镜像。
6. 成功或失败都会清理服务器临时依赖与构建目录；上一版源码路径、回滚镜像和发布提交会显示在日志中。

部署脚本会把当前环境 `.env` 中的 `WECHAT_MINI_APP_ID` 和 `WECHAT_MINI_APP_SECRET`
注入 `orderapp` 容器。生产与开发使用各自目录下的 `.env`，不会跨环境读取微信凭证。

构建期 API 地址固定按环境区分：development 使用
`https://dev.erp.qacoohee.com/app`，production 使用
`https://erp.qacoohee.com/app`。

### 2.2 功能分支远程预检

功能分支推送后、合入 `develop` 前执行：

```bash
./deploy_orderapp.sh --preflight development
```

该命令运行与发布相同的 Vue、小程序、Go 和隔离 Docker 构建门禁，但不会写入部署目录、Compose 文件、运行中容器或固定小程序目录。

### 2.3 执行开发环境部署
开发环境只能从 `develop` 分支部署：
```bash
chmod +x deploy_orderapp.sh
./deploy_orderapp.sh development
```

执行完成后，访问：
- `https://dev.erp.qacoohee.com/app/`
- `https://dev.erp.qacoohee.com/app/docs`

### 2.4 执行生产环境部署
生产环境只能从 `main` 分支部署：
```bash
chmod +x deploy_orderapp.sh
./deploy_orderapp.sh production
```

执行完成后，访问：
- `https://erp.qacoohee.com/app/`
- `https://erp.qacoohee.com/app/docs`

生产部署成功后，脚本会把服务器构建的正式小程序产物原子同步到：

```text
/Users/yiiiple-work/KFerp-miniapp-mp-weixin
```

同步只下载构建文件，不在 Mac 执行 npm。临时跳过下载可设置
`KFERP_SKIP_MINIAPP_EXPORT=1`；需要改到其他同名目录可设置
`KFERP_MINIAPP_EXPORT_DIR=/绝对路径/KFerp-miniapp-mp-weixin`。

每个正式包包含 `RELEASE_INFO`，记录 Git commit、API 地址、环境和构建时间。替换固定目录前，原目录会保留为
`KFerp-miniapp-mp-weixin.backup-时间-commit`；需要回滚本机预览包时，关闭微信开发者工具后把该备份目录恢复为固定目录即可。微信平台正式版回滚仍在微信公众平台按已发布版本操作。

> **发布边界：** `deploy_orderapp.sh` 只发布 ERP 服务并生成、同步小程序代码包；它不会上传微信平台、提交审核或发布正式版。还需要在微信开发者工具导入上面的固定目录，执行“上传”，再到微信公众平台完成审核和发布。服务器部署成功不代表用户微信里的小程序已更新。

---

## 3. 手动部署（仅用于故障恢复）

普通发布不得使用下列命令绕过构建锁、提交一致性检查和自动回滚。只有自动脚本不能运行，且已记录当前提交、备份与回滚点时才按本节排障。

### 3.1 同步 docs
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "mkdir -p /opt/stacks/erp/orderapp/docs"
COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata --exclude='._*' --exclude='*/._*' -C orderapp-remote/docs -cf - . | ssh -i openclaw_jj_ed25519 root@1.12.242.58 "tar -C /opt/stacks/erp/orderapp/docs -xf -"
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "mkdir -p /opt/stacks/erp/orderapp/docs/workspace"
scp -i openclaw_jj_ed25519 REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md DEPLOYMENT.md root@1.12.242.58:/opt/stacks/erp/orderapp/docs/workspace/
```

### 3.2 同步源码
```bash
scp -i openclaw_jj_ed25519 -r orderapp-remote/* root@1.12.242.58:/opt/stacks/erp/orderapp/
```

### 3.3 构建与重启
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "cd /opt/stacks/erp && docker compose build orderapp && docker compose up -d"
```

### 3.4 查看状态/日志
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "cd /opt/stacks/erp && docker compose ps"
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "docker logs --tail=200 erp_orderapp"
```

---

## 4. 常见问题（FAQ）

### Q0: Mac 内存或磁盘不足，需要先在本地构建吗？

不需要，也不应这样做。发布入口只在本机读取 Git 元数据并传输源码；`npm ci`、测试、类型检查、Vue/UniApp 构建、Go 测试和 Docker 构建都在服务器临时目录串行完成。

### Q0.1: ERP 已发布，为什么微信里仍是旧页面？

ERP 服务发布和微信小程序发布是两条链路。确认生产脚本已更新固定目录
`/Users/yiiiple-work/KFerp-miniapp-mp-weixin`，再用微信开发者工具导入该目录、清缓存并编译，然后上传新版本、提交审核并发布。只重启服务器容器不会更新微信客户端代码。

### Q0.2: 提示 another KFerp build or deployment is running

服务器发布锁正在保护另一批构建或部署。等待上一批结束后重试，不要手工删除锁或并行启动第二个 Docker/npm 构建。

### Q1: 页面 401 未授权
orderapp 走 BasicAuth：
- 用户名默认：`order`
- 密码来自服务器 `/opt/stacks/erp/.env` 的 `ORDERAPP_PASS`
- 如需临时关闭外层 BasicAuth（例如 in-app browser 无法处理原生 BasicAuth 弹窗），在 `/opt/stacks/erp/.env` 设置 `DISABLE_BASIC_AUTH=true` 后重建/重启 `orderapp`。该开关只取消外层 BasicAuth，系统账号/Bearer 登录仍由应用权限体系控制；使用完应恢复为 `false` 并重新部署。

### Q2: push 到 GitHub 失败（Repository not found）
通常是 SSH Key 权限问题：
- 如果 `ssh -T git@github.com` 显示 `Hi Backstage2539!` 才是账号级别 key
- 如果显示 `Hi Backstage2539/<repo>!` 说明你加成了 deploy key（只对单仓库生效）

### Q3: 容器启动失败
先看 compose 状态 + 日志：
```bash
docker compose ps
docker logs --tail=200 erp_orderapp
```

自动发布不会把 `Running=true` 当成服务已经就绪：新容器必须在最长 120 秒内连续三次通过容器内 HTTP 探测，环境 URL 还会在短暂 502 或容器地址切换时限内重试。探测最终失败会先保留新容器日志，再恢复上一版服务器源码和镜像；终端中的 `previous_source` 与 `rollback_image` 是本次回滚证据。

---

## 5. 建议的发布节奏
- 功能分支完成并推送后，先执行 `./deploy_orderapp.sh --preflight development`。服务器会持锁、限资源、串行完成 Vue/小程序/Go 测试和隔离 Docker 构建；不会写入部署目录、替换服务镜像、重启容器或同步本机小程序目录。
- 预检通过后，功能分支再合入 `develop`，执行 `./deploy_orderapp.sh development`，由服务器重复门禁并完成开发环境部署。
- 开发环境验收后合入 `main`，执行 `./deploy_orderapp.sh production`。
- 生产脚本结束后，在微信开发者工具使用固定目录单独完成预览、上传、审核和正式发布，并记录服务器提交、小程序新旧版本与回滚点。
