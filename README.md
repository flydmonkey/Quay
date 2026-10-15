# Quay

Quay（码头）是一套跑在浏览器里的个人运维台。本机和远程主机都靠在同一处：终端、文件、SQL、MongoDB、Redis，以及 VNC 远程桌面。

打开页面后需要登录。没有设置账号时，用户名和密码都是 `admin`。

Go 模块名仍是 `webadmin`，页面和接口里的产品名是 Quay。

## 功能

- **终端**：浏览器里的 xterm。不带主机参数时打开本机 PTY；带上已保存的 SSH 主机则进入远程 shell，并支持调整行列数。
- **文件**：浏览、预览、编辑、上传、下载、新建目录、删除。可操作本机目录，也可通过 SFTP 操作已保存的 SSH 主机。
- **SQL**：保存 MySQL / PostgreSQL 连接，查看库表结构并执行查询。
- **MongoDB**：按连接 URI 管理文档库，浏览库与集合，按过滤条件查询文档。
- **Redis**：保存单机或集群连接，浏览 key、查看类型、执行命令。
- **VNC**：保存连接并用 noVNC 打开远程桌面。服务端把浏览器 WebSocket 转发到目标 VNC 端口（默认 `5900`）。

连接信息写在进程工作目录下的 `data/*.json`。列表接口会对密码和私钥做脱敏，文件里仍是明文。

## 技术栈

| 层 | 选型 |
| --- | --- |
| 服务端 | Go 1.23、Gin |
| 前端 | Vue 3、Vue Router、Vite、TypeScript |
| 终端 | xterm.js、本地 PTY 或 SSH |
| 远程文件 | SFTP |
| 远程桌面 | noVNC，WebSocket 转发到 VNC TCP |

## 环境要求

- Go 1.23 或更高版本
- Node.js（仅在开发或构建前端时需要）
- 本机终端需要可用的 shell（默认读取 `SHELL`，否则使用 `/bin/sh`）

## 开发

开两个终端，都在仓库根目录操作。

```bash
go run ./cmd/server
```

```bash
cd web
npm install
npm run dev
```

前端开发服务器在 <http://localhost:5173>，并把 `/api`（含 WebSocket）代理到 <http://localhost:8080>。

## 构建后运行

```bash
cd web
npm install
npm run build
cd ..
go run ./cmd/server
```

`web/dist` 存在时，同一进程会托管页面。打开 <http://localhost:8080>。

## 打包

推送到 `main`，或在 GitHub 的 Actions 页手动运行 **Package**，会跑测试并打出四个压缩包：

- `quay-linux-amd64.tar.gz`
- `quay-linux-arm64.tar.gz`
- `quay-darwin-amd64.tar.gz`
- `quay-darwin-arm64.tar.gz`

每个包里是对应系统的 `quay` 可执行文件，以及已经构建好的 `web/dist`。在 Actions 的运行记录里下载，解压后进入该目录执行：

```bash
./quay
```

程序从当前目录读取 `web/dist`，所以要在解压出来的目录里启动。默认监听 `8080`。

## 配置

| 变量 | 作用 | 默认 |
| --- | --- | --- |
| `PORT` | HTTP 监听端口 | `8080` |
| `APP_ENV` | 仅在 `GET /api/info` 中回显 | 空 |
| `QUAY_USER` | 登录用户名 | `admin` |
| `QUAY_PASSWORD` | 登录密码 | `admin` |
| `QUAY_SESSION_SECRET` | 会话 Cookie 的签名密钥。更换或留空重启后需要重新登录 | 启动时随机生成 |
| `WEBADMIN_FS_ROOT` | 本机文件管理允许访问的根目录 | 进程当前工作目录 |
| `SHELL` | 本地终端使用的 shell | `/bin/sh` |

终端 WebSocket 也可以用查询参数 `shell` 指定本次会话的 shell。

## 数据文件

服务端在当前工作目录维护这些文件（不存在时按空列表处理）：

| 文件 | 内容 |
| --- | --- |
| `data/ssh_hosts.json` | SSH 主机（密码或私钥） |
| `data/db_connections.json` | MySQL / PostgreSQL |
| `data/mongo_connections.json` | MongoDB URI |
| `data/redis_connections.json` | Redis |
| `data/vnc_connections.json` | VNC |

这些文件含有凭据，不要提交到仓库，也不要暴露到不可信网络。

## 接口一览

除健康检查、登录和退出外，业务接口都在 `/api` 下，并且需要登录。

| 路径 | 说明 |
| --- | --- |
| `GET /api/health` | 健康检查，不需要登录 |
| `POST /api/login` | 登录。请求 JSON：`{"username","password"}` |
| `POST /api/logout` | 退出并清除会话 Cookie |
| `GET /api/auth/me` | 当前登录用户，未登录返回 401 |
| `GET /api/info` | 应用名与环境 |
| `/api/fs/*` | 本机文件 |
| `/api/ssh/hosts` | SSH 主机的增删查 |
| `/api/ssh/fs/*` | 远程文件（SFTP） |
| `GET /api/terminal` | 终端 WebSocket；`?host=<id>` 走 SSH |
| `/api/db/*` | SQL 连接、结构树、查询 |
| `/api/mongo/*` | MongoDB 连接、结构树、查询 |
| `/api/redis/*` | Redis 连接、key、命令 |
| `/api/vnc/connections` | VNC 连接的增删查 |
| `POST /api/vnc/check` | 检测 VNC 地址是否可达 |
| `GET /api/vnc/ws` | VNC WebSocket 代理，参数 `host`、`port` |

## 目录

```
cmd/server/          入口
internal/auth/       登录与会话 Cookie
internal/terminal/   本地 PTY 与 SSH 终端
internal/fs/         本机文件
internal/ssh/        SSH 主机与连接
internal/sshfs/      SFTP
internal/db/         MySQL / PostgreSQL
internal/mongo/      MongoDB
internal/redis/      Redis
internal/vnc/        VNC 连接与 WebSocket 代理
web/                 Vue 前端
data/                运行时连接配置
```

## 安全

除健康检查、登录、退出和登录页所需的静态文件外，页面和接口都要先登录。会话放在名为 `quay_session` 的 HttpOnly Cookie 里，有效期 7 天。

没有设置 `QUAY_USER` 和 `QUAY_PASSWORD` 时，账号是 `admin` / `admin`。在别人能访问到的机器上，启动前请改掉这两个值，并设置 `QUAY_SESSION_SECRET`。

登录只挡住 Quay 自己。已经保存的主机和数据库凭据仍然以明文写在 `data/`。请只在本机或受信任的网络里运行。
