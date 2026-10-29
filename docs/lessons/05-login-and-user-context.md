# 第 05 课：登录认证、持久会话与用户上下文

日期：2026-10-29。状态：真实 HTTP + PostgreSQL 验证完成；尚无聊天与会话业务 API。

## 本课目标

把数据库、Handler、Service 和可信用户上下文串起来。用户先登录，后续请求携带 token；服务端验证 token 后才允许访问用户数据。保持简单项目结构，不为认证创建一套通用仓储或权限框架。

## Ragent 源码对照

参考本地 Ragent：AuthController 的 POST /auth/login、POST /auth/logout，UserController 的 GET /user/me；AuthServiceImpl 校验账号后使用 SaToken，SaTokenConfig 注册登录检查，UserContextInterceptor 把登录用户写入 UserContext。

当前参考源码的 passwordMatches 使用 stored.equals(input)。Go 版不照搬明文密码比较，改用 golang.org/x/crypto/bcrypt。保留路径、Authorization 直接传登录 token，以及 HTTP 200 + code 的响应约定。

Ragent 登录返回 userId/role/token/avatar，当前 Go 版只提供真实已实现的 userId/token；/user/me 返回 userId/username。角色、头像、后台用户管理后续实现，不伪造管理员权限，也不声称能无修改接原前端。

## 代码链路

```text
POST /auth/login
  → handler/auth.go：JSON 校验、请求限流
  → service/AuthService.Login：查用户、校验密码、生成并保存会话
  → JSON Result（token 只在登录响应返回）

GET /user/me 或 POST /auth/logout
  → RequireLogin：读取 Authorization、验证数据库会话
  → common.WithUser(request.Context(), user)
  → Handler/Service 从同一 context 取得可信用户
```

internal/service/auth.go 直接使用 GORM，用户映射结构只在此服务使用，暂时保持私有。internal/handler/auth.go 负责协议与中间件，internal/config/auth.go 负责统一配置。没有额外的 Repository/Manager/Provider 层。

## 密码与账号创建

不开放匿名注册。操作员在项目目录运行：

```sh
make migrate
make user-create USERNAME=alice
```

终端会提示输入新密码，不回显。不要把密码写在命令参数里。自动化可通过进程环境 RAGENT_USER_PASSWORD 传入；不建议长期存入 .env，也不要写进 shell 历史或脚本。

用户名为 3..64 位 ASCII 字母数字及 _.-，首字符必须字母或数字，大小写敏感；创建密码为 12..72 字节、不能全空白，不自动 trim。bcrypt 自带随机盐，当前使用 DefaultCost=10。72 是字节上限，中文通常占多个字节；拒绝超长输入，绝不静默截断。

用户名有唯一约束。命令只能创建，重复用户名报错，不覆盖密码，不给既有用户自动重置密码。当前数据库中没有预置管理员或公开默认密码。

## 为什么用数据库会话，而不是 JWT

每次成功登录生成 32 字节密码学随机 token，编码为 43 字符 URL-safe 字符串。PostgreSQL 只保存其 SHA-256 摘要、user_id 和 expires_at；这是随机 token 的索引摘要，不是用 SHA-256 替代密码散列。

认证时计算摘要并关联用户，使用数据库当前时间检查有效期。会话与应用进程分离，所以重启后 token 仍可验证。登出删除当前会话，立即影响后续请求；同用户其他会话不会自动登出。已通过验证的在途请求不会被强制撤销。

当前不需要 JWT 签名、刷新 token 或额外 Redis。密码散列、随机数、限流算法分别使用 Go 官方组件，没有自行实现密码算法。会话生命周期由本项目用几条明确的数据库操作管理，不是自研一套认证框架。

## 接口实际使用

数据库开启且完成第 2 版迁移后运行 make run。

```text
POST /auth/login
Content-Type: application/json

{"username":"alice","password":"你的密码"}
```

成功形状：

```json
{"code":"0","data":{"userId":"服务端用户ID","token":"本次登录token"},"requestId":"请求标识"}
```

之后请求：

```text
GET /user/me
Authorization: 本次登录token
```

```text
POST /auth/logout
Authorization: 本次登录token
```

Authorization 直接放 token，不加 Bearer，也不从 query 或客户端 userId 获取身份。普通错误使用 HTTP 200 的 JSON Result，code=A000001 表示客户端错误，B000001 表示服务暂不可用；成功码是字符串 "0"。响应 Cache-Control=no-store，禁止缓存登录响应与当前用户信息。

数据库关闭时只提供健康检查，认证路由不注册；数据库开启但认证结构未迁移时，服务在监听端口前失败，提示执行 make migrate。/healthz 的存活语义保持不变。

## 安全边界

不存在的账号与错误密码统一提示，未找到用户时也执行一次 dummy bcrypt 校验，减少基于耗时的账号枚举；这不是绝对恒定时间保证。请求 JSON 限制 4 KiB，拒绝未知字段和多个顶层 JSON 值。数据库查询使用参数绑定，原始数据库错误不返回客户端。

登录按真实客户端 IP 使用 golang.org/x/time/rate 限流：默认每分钟补充 10 次，最多突发 10 次，成功/失败都消耗名额。最多保存 1024 个 IP 状态，惰性清理；容量满时拒绝新来源。当前不信任代理头，避免通过伪造 X-Forwarded-For 绕过限制。多实例统一限流、账户维度治理和可信代理配置仍在后续课程。

默认会话 24h；认证查询默认 5s 期限。每次登录最多清理 100 条已过期会话，有效性判断不依赖清理任务。尚无密码修改、全端登出、会话数量上限与独立过期清理任务，后续按需求补齐。

Go bcrypt 运算本身不能被 context 强制中断；请求取消/超时会阻止随后数据库操作，不承诺密码计算瞬停。登录限流降低计算滥用，仍需部署层的总连接与请求治理。

HTTP 使用 header token，不设置认证 Cookie；未来前端应避免将 token 放入 URL、日志或永久 localStorage，浏览器存储/CSRF策略在前端课程明确。部署必须使用 HTTPS，本机回环开发除外。

## 真实验证

先启动本项目专属 PostgreSQL，并执行迁移。然后：

```sh
make auth-smoke
```

该命令从本项目 ragent-go-study-postgres-1 容器取得临时连接设置，不修改 .env，不操作其他项目数据库。创建两个随机测试账号，启动真实服务，验证正常/错误登录、严格 JSON、限流、用户隔离、伪造/过期/撤销 token、重启保留、密码散列、token 摘要及日志无凭据。最后仅删除自己的随机账号，会话由外键级联清理。它不是生产数据库上的检测命令。

本次实测全部通过；已有 race 回归、vet、离线冒烟与模块校验也通过。测试只聚焦关键行为，没有为字段传递添加镜像测试；Python 进程集成不计入 go test 的覆盖率，不将覆盖率数字当作本课结论。

**本机配置现状**：执行本课时发现 .env 的 RAGENT_DB_ENABLED=false 且密码为空，已保留未改。验证使用专属容器的临时参数。要手动使用登录接口，请先配置正确的数据库连接、设置 RAGENT_DB_ENABLED=true，再执行 make migrate 和 make run。

## 阅读顺序与复盘

1. migrations/00002_create_auth_sessions.sql：会话与用户的关系、有效期和索引。
2. service/auth.go：账号创建、Login、Authenticate、Logout。
3. handler/auth.go：JSON、Result、RequireLogin 与限流。
4. common/user.go：可信身份如何随标准 context 传到底层。
5. cmd/server/main.go 与 cmd/usercreate/main.go：真实依赖装配和账号初始化。

解释：为什么数据库不存原始 token？为什么登出只影响当前会话？为什么 Header 里的 token 仍然是不可信输入？为什么不能允许模型传 userId？为什么不照搬参考项目的明文密码比较？

下一课：会话与消息持久化，基于已认证的用户实现真实会话 Service；第 07 课再把 Agent 接到 Ragent 风格 SSE。

## 官方依据

- [Go bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt)
- [Go rate limiter](https://pkg.go.dev/golang.org/x/time/rate)
- [GORM Context](https://gorm.io/docs/context.html)
