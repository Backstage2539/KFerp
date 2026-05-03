# DEPLOYMENT - KFerp 部署流程与教程

> 目标：把"从本地代码到线上可访问"这条链路写成可重复执行的步骤。
> 
> 环境约定（截至 2026-05-03）：
> - 正式环境 `production`：未来线上正式发布目标，默认由 `./deploy_orderapp.sh` 发布。
> - 开发环境 `development`：保留原 develop 发布链路，由 `./deploy_orderapp.sh development` 发布。
> - 开发环境不作为未来线上正式发布目标。

当前正式环境规划：
- 域名：`https://erp.qacoohee.com/`
- 服务器：`root@1.12.242.58`
- 部署目录：`/opt/stacks/erp-production`
- 发布分支：`main` / `origin/main`

当前开发环境：
- 服务器：`root@1.12.242.58`
- 部署目录：`/opt/stacks/erp`
- 发布分支：`develop` / `origin/develop`
- 现有容器：`erp_orderapp`、`erp_caddy`、`erp_postgres`

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
服务器 `/opt/stacks/erp` 下使用 `docker compose` 管理服务，需要：
- Docker Engine
- Docker Compose v2（`docker compose` 子命令）

### 0.3 初始化正式环境目录
正式环境第一次部署前，服务器需要先准备 `/opt/stacks/erp-production`：

```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "mkdir -p /opt/stacks/erp-production"
scp -i openclaw_jj_ed25519 deploy/production/docker-compose.yml deploy/production/Caddyfile root@1.12.242.58:/opt/stacks/erp-production/
scp -i openclaw_jj_ed25519 deploy/production/.env.example root@1.12.242.58:/opt/stacks/erp-production/.env
```

然后登录服务器编辑 `/opt/stacks/erp-production/.env`，把 `POSTGRES_PASSWORD` 和 `ORDERAPP_PASS` 改成正式环境真实值。不要把真实 `.env` 回传或提交到仓库。

注意：正式环境模板里的 Caddy 绑定 `443`。同一台服务器上如果开发环境 Caddy 也在绑定 `443`，正式切换前需要安排入口切换窗口；不要在未确认切换方案时同时启动两个绑定 `443` 的 Caddy。

---

## 1. 环境结构说明（你在服务器上会看到什么）

登录服务器：
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58
```

正式环境核心目录：
- `/opt/stacks/erp-production/docker-compose.yml`：正式环境 compose 定义
- `/opt/stacks/erp-production/.env`：正式环境变量（包含数据库密码、ORDERAPP_PASS 等）
- `/opt/stacks/erp-production/orderapp/`：正式环境 orderapp 源码（会被脚本覆盖同步）
- `/opt/stacks/erp-production/orderapp_data/`：正式环境 orderapp 持久化数据（assets 等）
- `/opt/stacks/erp-production/postgres_data/`：正式环境 Postgres 持久化数据

开发环境核心目录：
- `/opt/stacks/erp/docker-compose.yml`：开发环境 compose 定义
- `/opt/stacks/erp/.env`：开发环境变量（包含数据库密码、ORDERAPP_PASS 等）
- `/opt/stacks/erp/orderapp/`：开发环境 orderapp 源码（会被脚本覆盖同步）
- `/opt/stacks/erp/orderapp_data/`：开发环境 orderapp 持久化数据（assets 等）
- `/opt/stacks/erp/postgres_data/`：开发环境 Postgres 持久化数据

---

## 2. 一键部署（推荐）

仓库根目录有脚本：`deploy_orderapp.sh`

`./deploy_orderapp.sh` 默认发布正式环境。正式发布必须在 `main` 分支执行，并且本地 `HEAD` 必须等于 `origin/main`。

开发环境发布必须显式指定：

```bash
./deploy_orderapp.sh development
```

开发环境发布仍要求在 `develop` 分支执行，并且本地 `HEAD` 必须等于 `origin/develop`。这保留原开发环境链路，不把 develop 发布当作正式线上发布。

发布前可先查看计划，不执行构建或 SSH 写入：

```bash
./deploy_orderapp.sh --print-plan
./deploy_orderapp.sh --print-plan development
```

### 2.1 脚本做了什么
1) 同步 docs（用于 `/app/docs` 页面展示）：
- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
- `HOW_TO_WORKFLOW.md`
- `DEPLOYMENT.md`（本教程）

2) 同步应用源码：
- 把 `orderapp-remote/*` 拷贝到服务器 `/opt/stacks/erp/orderapp/`

3) 重建并滚动更新容器：
- `docker compose build orderapp`
- `docker compose up -d`

### 2.2 执行正式环境部署
在本机（workspace 根目录）执行：
```bash
chmod +x deploy_orderapp.sh
./deploy_orderapp.sh
```

执行完成后，访问：
- `https://erp.qacoohee.com/app/`
- `https://erp.qacoohee.com/app/docs`

### 2.3 执行开发环境部署
仅在需要更新开发环境时执行：

```bash
chmod +x deploy_orderapp.sh
./deploy_orderapp.sh development
```

---

## 3. 手动部署（用于排障/理解）

### 3.1 同步 docs
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "mkdir -p /opt/stacks/erp-production/orderapp/docs"
scp -i openclaw_jj_ed25519 REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md DEPLOYMENT.md root@1.12.242.58:/opt/stacks/erp-production/orderapp/docs/
```

### 3.2 同步源码
```bash
scp -i openclaw_jj_ed25519 -r orderapp-remote/* root@1.12.242.58:/opt/stacks/erp-production/orderapp/
```

### 3.3 构建与重启
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "cd /opt/stacks/erp-production && docker compose build orderapp && docker compose up -d"
```

### 3.4 查看状态/日志
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "cd /opt/stacks/erp-production && docker compose ps"
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "docker logs --tail=200 erp_orderapp"
```

---

## 4. 常见问题（FAQ）

### Q1: 页面 401 未授权
orderapp 走 BasicAuth：
- 用户名默认：`order`
- 密码来自服务器 `/opt/stacks/erp/.env` 的 `ORDERAPP_PASS`

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
- 小改动：先合入 `develop` 做开发环境验证，需要正式上线时再将确认过的版本推进 `main`，然后执行 `./deploy_orderapp.sh`。
- 大改动：先在本地/开发环境跑一遍（特别是数据库变更），再部署正式环境。
- 开发环境只用 `./deploy_orderapp.sh development` 更新；不要把 develop 发布当作正式线上发布。
