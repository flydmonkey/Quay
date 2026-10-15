# Quay 登录设计

未登录不能使用 Quay 的功能页和受保护接口。单账号，HttpOnly 签名 Cookie。

## 账号

环境变量优先，两个变量各自回退，互不影响：

| 变量 | 未设置时 |
| --- | --- |
| `QUAY_USER` | `admin` |
| `QUAY_PASSWORD` | `admin` |
| `QUAY_SESSION_SECRET` | 进程启动时随机生成。重启后已有 Cookie 失效，需要重新登录 |

密码比较使用常量时间比较。日志、错误信息和响应里不出现密码。

## 认证范围

公开：

- 登录页需要的静态文件：`/`、`/assets/*`、`/favicon.ico`，以及前端路由回退到 `index.html`
- `GET /api/health`
- `POST /api/login`
- `POST /api/logout`（没有有效 Cookie 时也接受，只负责清除 Cookie）

其余 `/api` 都要有效 Cookie。`/api/terminal` 和 `/api/vnc/ws` 在 Gin 中间件之外升级 WebSocket，升级前用同一套校验。没有有效 Cookie 时返回 401，不升级。

## Cookie

- 名称：`quay_session`
- `HttpOnly`、`Path=/`、`SameSite=Lax`
- 仅当 `http.Request.TLS != nil` 时设置 `Secure`。本机 HTTP 不加 `Secure`，否则浏览器不会保存。不读取 `X-Forwarded-Proto`
- 有效期 7 天，同时写入 `Max-Age` 和签名载荷里的过期时间
- 只有一个账号，Cookie 不携带用户名。值的格式是 `base64url(过期时间的十进制秒)` + `.` + `base64url(HMAC-SHA256)`。HMAC 的密钥是会话密钥，消息是过期时间的十进制秒
- 过期、签名不对、格式不对，都视为未登录
- 退出时用同名、同路径的 Cookie 覆盖，`Max-Age=0`

## 接口

`POST /api/login`

请求 JSON：`{"username":"...","password":"..."}`。

成功：200，`{"ok":true}`，并设置 Cookie。

失败（用户名为空、密码为空、不匹配、JSON 无法解析）：401，`{"error":"用户名或密码错误"}`。不区分是哪一种失败。

`POST /api/logout`

200，`{"ok":true}`，清除 `quay_session`。

`GET /api/auth/me`

已登录：200，`{"username":"<QUAY_USER 或 admin>"}`。

未登录：走统一的 401，`{"error":"未登录"}`。这个接口不单独放行。

其他受保护接口在未登录、Cookie 过期或签名无效时同样返回 401，`{"error":"未登录"}`。

## 页面

新增路由 `/login`。未登录时访问任何功能路由都先请求 `GET /api/auth/me`，401 则进入 `/login`。已登录时访问 `/login` 则回到 `/`。

登录页只显示 Quay 标题、用户名、密码和登录按钮，不显示终端、文件、数据库、Redis、VNC 入口。样式沿用现有深色界面。

登录成功后进入首页。已登录时顶栏显示「退出」。退出调用 `POST /api/logout`，然后进入 `/login`。

现有页面继续用 `fetch` 调用接口。在应用入口包装 `window.fetch`：除 `POST /api/login` 外，`/api` 返回 401 时进入 `/login`。终端和 VNC 的 WebSocket 若无法建立，再请求 `GET /api/auth/me`，得到 401 时同样进入 `/login`。

## 代码位置

- `internal/auth`：读取配置、签发和校验 Cookie、登录、退出、`me`、Gin 中间件，以及给原始 `http.Request` 用的校验函数
- `cmd/server/main.go`：挂上中间件；在 WebSocket 分支里先校验再升级
- `web/src/views/Login.vue`：登录页
- `web/src/router/index.ts`：路由和进入前的会话检查
- `web/src/App.vue`：已登录时的退出按钮
- `web/src/main.ts`：包装 `fetch`，处理 401
- `web/src/views/Terminal.vue`、`web/src/views/Vnc.vue`：WebSocket 建立失败后的会话检查
- `README.md`：补充上述三个环境变量，并写明未配置时的默认账号是 `admin` / `admin`

## 测试

Go 测试覆盖：

- 正确账号签发的 Cookie 能通过校验
- 过期、篡改、格式错误的 Cookie 被拒绝
- 错误密码、空用户名、空密码登录失败，且响应不包含正确密码
- 中间件放行 `POST /api/login`、`POST /api/logout`、`GET /api/health`，并拦住其他 `/api`
- 没有有效 Cookie 时，`/api/terminal` 和 `/api/vnc/ws` 返回 401 且不升级

实现完成后在浏览器里确认：未登录打开功能页会到登录页；使用 `admin` / `admin` 能进入首页并看到功能入口；退出后再次请求受保护接口返回 401。
