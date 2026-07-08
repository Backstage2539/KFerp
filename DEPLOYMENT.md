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
- 本地有私钥（本项目默认使用 workspace 内的 key）：
  - `openclaw_jj_ed25519`
- 本地有基础工具：
  - `ssh` / `scp`
  - `docker`（仅服务器需要，本地不需要）

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
  - `.env`：环境变量（包含数据库密码、ORDERAPP_PASS 等）
  - `orderapp/`：orderapp 源码（会被脚本覆盖同步）
  - `orderapp_data/`：orderapp 持久化数据（assets 等）
  - `postgres_data/`：Postgres 持久化数据

---

## 2. 一键部署（推荐）

仓库根目录有脚本：`deploy_orderapp.sh`

### 2.1 脚本做了什么
1) 同步 docs（用于 `/app/docs` 页面展示）：
- `orderapp-remote/docs/` 作为线上 `/app/docs` 的唯一来源，包含操作手册、需求和验收文档
- 根目录 `REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`HOW_TO_WORKFLOW.md`、`DEPLOYMENT.md` 仅作为构建期治理上下文放入 `docs/workspace/`
- 根目录不再同步 `OPERATION_MANUALS.md` 或 `OP_MANUAL_*.md`，避免两份手册人工镜像

2) 同步应用源码：
- production：把 `orderapp-remote/*` 拷贝到服务器 `/opt/stacks/erp-production/orderapp/`
- development：把 `orderapp-remote/*` 拷贝到服务器 `/opt/stacks/erp/orderapp/`

3) 重建并滚动更新容器：
- `docker compose build orderapp`
- `docker compose up -d`

### 2.2 执行开发环境部署
开发环境只能从 `develop` 分支部署：
```bash
chmod +x deploy_orderapp.sh
./deploy_orderapp.sh development
```

执行完成后，访问：
- `https://dev.erp.qacoohee.com/app/`
- `https://dev.erp.qacoohee.com/app/docs`

### 2.3 执行生产环境部署
生产环境只能从 `main` 分支部署：
```bash
chmod +x deploy_orderapp.sh
./deploy_orderapp.sh production
```

执行完成后，访问：
- `https://erp.qacoohee.com/app/`
- `https://erp.qacoohee.com/app/docs`

---

## 3. 手动部署（用于排障/理解）

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

---

## 5. 建议的发布节奏
- 小改动：直接 `./deploy_orderapp.sh`
- 大改动：先在本地/测试环境跑一遍（特别是数据库变更），再部署线上
