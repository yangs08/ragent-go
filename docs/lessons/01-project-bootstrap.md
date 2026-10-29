# 第 01 课：项目初始化与 Eino 兼容性验证

- 创建：2026-10-27；本次实现与验收：2026-10-28。
- 状态：本课代码与本地验收完成；真实供应商正常调用已验证，学员练习及真实取消/异常场景待完成。
- 所属里程碑：M0/M1；关联 C02、C20、C21、C23 的基础部分，不表示这些完整平台能力已交付。
- 代码目录：`/Users/admin/workspace/ragent-go/`。
- 前置阅读：[项目设计](../PROJECT.md)。

路径说明：命令中的 `/path/to/ragent-go` 请替换为自己的项目目录；本机环境记录保留历史实际路径。课程正式文档位于项目 `docs/lessons/`。

## 1. 本课成果与边界

现在有两条可以复现的链路：

```text
HTTP：配置 → 日志/监听器装配 → /healthz → 请求标识 → SIGTERM → 限时退出
模型：配置 → Eino ChatModel → 兼容协议请求 → 逐块消费 → 完成/超时/取消/错误
```

模型通过独立 `modelcheck` 命令验证，尚未接入 HTTP 聊天入口。没有对外开放未鉴权的模型接口。真实模型普通文本调用已成功验证；自动化回归继续使用本地模拟供应商，不重复产生模型费用。没有数据库、会话、Agent 或前端。

本课目标是掌握企业项目最底层的可配置、可启动、可测试、可退出机制，并验证 Eino 与模型扩展能在当前工具链一起工作。

## 2. 为什么这样组织工程

```text
ragent-go/
  cmd/server/                     HTTP 进程入口与装配测试
  cmd/modelcheck/                 独立 Eino 模型联调入口与测试
  internal/config/               服务配置与校验
  internal/bootstrap/            HTTP 运行与退出生命周期
  internal/handler/       路由、健康检查、请求日志
  internal/model/                模型配置与 Eino 流式适配验证
  configs/README.md              .env 与环境变量说明
  .env / .env.example            本机配置（忽略提交）/无凭据模板
  scripts/smoke.py               两个真实进程的本地冒烟验证
  go.mod / go.sum                固定依赖与完整性信息
  Makefile                       统一运行/检查入口
  .cache/                        模块与构建缓存，不入版本控制
  bin/                           构建产物，不入版本控制
```

关键原则：入口创建依赖，业务包使用依赖；配置在监听端口前校验。没有为未来功能一次创建大量空包，也没有引入暂时用不到的数据库和容器。

`module ragent-go` 是本地模块名，尚未绑定远程仓库。将来确定仓库地址后再统一调整导入路径。

## 3. Ragent 源码导读

参考提交：`c3b61577`；源码根目录：`/Users/admin/workspace/ragent`。

1. `pom.xml`：观察模块与版本管理；Go 对应 go.mod/go.sum 和职责分包，不需要逐个 Maven 模块翻译。
2. `infra-ai/src/main/java/com/nageoffer/ai/ragent/infra/chat/ChatClient.java`：观察同步、流式、回调与取消句柄，理解流式调用不只返回一个字符串。
3. `agent/src/main/java/com/nageoffer/ai/ragent/agent/config/AgentEngineConfiguration.java`：观察配置检查和模型装配。

本课不照搬 Java 的依赖注入容器。Go 入口通过构造参数显式装配；Eino 模型组件负责协议，应用负责配置校验、截止时间、消费和错误边界。

## 4. 版本与环境证据

| 项目 | 本次实测结果 |
|---|---|
| 操作系统架构 | darwin/arm64 |
| Go | 本机命令 `/Users/admin/.local/bin/go`，`go version go1.27.3 darwin/arm64`；GOROOT 为 `/Users/admin/.local/share/go/1.27.3` |
| HTTP 框架 | Gin v1.12.0 |
| Eino 核心 | v0.9.22 |
| Eino 模型扩展 | github.com/cloudwego/eino-ext/components/model/openai v0.1.13 |
| Git | /usr/bin/git，2.39.5；用户已初始化仓库，第一课提交为 main 分支 c607fe7 |
| Docker | 已定位客户端；本课不依赖容器，daemon 未验证 |

工具链从官方二进制地址下载，SHA-256 与本次访问官方元数据一致后才解压：

```text
go1.27.3.darwin-arm64.tar.gz
SHA-256: dd3908cf20959d798c1e06ebe5b72213f0505ce01373b40ce3110975a6cd99c2
```

来源：[Go 官方下载元数据](https://go.dev/dl/?mode=json)、[安装说明](https://go.dev/doc/install)。工具链最初位于工程内部，2026-10-28 按用户要求迁入本机用户级公共目录 `/Users/admin/.local/share/go/1.27.3`，通过现有 PATH 下的 `/Users/admin/.local/bin/go` 与 `gofmt` 提供命令。无需修改 shell 配置，其他项目也可直接调用，Makefile 默认使用 `go`。其他机器可使用兼容 Go，通过 Make 的 GO 参数指定。

官方主站/模块代理曾连接超时：工具链改用官方 `dl.google.com` 下载地址；模块使用 `https://goproxy.cn` 成功下载，保留 Go 校验机制。代理只传输公开依赖请求，没有发送项目源码或凭据。

版本接口依据为下载后的固定版本源码：Eino `components/model/interface.go`、`schema/stream.go`，模型扩展 `chatmodel.go`。扩展的 [官方 README](https://github.com/cloudwego/eino-ext/blob/main/components/model/openai/README.md) 用于导读，编译以锁定版本为准。

本轮只证明这一核心/模型组合在 darwin/arm64 的本地编译与协议测试通过。ADK、MCP、Checkpoint、Skills 与追踪扩展的完整组合仍需后续验证。

## 5. 先理解代码的四个边界

### 5.1 配置：错误尽量在启动时发现

先读 `internal/config/config.go` 和 `cmd/server/main.go`。

- 入口用 `Environment(".env", os.LookupEnv)` 构建配置读取函数；使用 godotenv v1.5.1 解析文件，系统环境变量优先。
- `Load(getenv)` 接收该读取函数，负责类型解析与字段校验；测试可以使用字典，不依赖真实配置或凭据。
- 默认读取当前工作目录的 `.env`，所以本地从工程根目录运行；缺失文件可回退，文件语法/读取错误会阻止启动。
- 校验监听地址、端口、退出超时和日志级别；错误仅说明字段规则，不输出原始配置值。
- 默认 `127.0.0.1:8080`，仅本机可访问。
- 配置通过后再创建日志和监听器，最后进入 Serve。

先读 `internal/config/environment.go`，再读 `config.go`。加载器返回读取闭包，不修改进程环境；文件内容不在错误中回显。真实 `.env` 被 Git 忽略，模板 `.env.example` 不包含真实凭据。

应用配置中不直接持有全局单例。注入读取函数让测试无需修改真实环境，也避免某个业务包悄悄读取不同配置。

### 5.2 HTTP：健康检查不代表所有业务就绪

再读 `internal/handler/handler.go`。

- `/healthz` 表示 HTTP 进程能响应；未检查模型或数据库。
- 服务端生成 X-Request-ID，不直接信任客户端传入的任意字符串。
- JSON 日志记录请求标识、方法、路径与耗时，不记录查询参数或请求正文。
- 当前仅有健康检查；未来还需要独立 readiness、鉴权、资源限额和业务错误契约。

当前请求标识只覆盖 HTTP 请求日志。模型验证命令独立运行，尚未实现 HTTP 到模型的 Trace/请求标识透传。

### 5.3 生命周期：先等待，再在期限内结束

读 `internal/bootstrap/server.go`。

1. Serve 在 goroutine 中接收请求，主流程等待服务错误或取消信号。
2. SIGINT/SIGTERM 触发入口 context 取消。
3. 使用独立的退出 context 调用 Shutdown，停止接收新请求并等待在途请求。
4. 超过退出预算后调用 Close，取消仍活动的连接并返回超时错误。

为什么退出 context 不能直接复用已经取消的父 context？因为它会立即失效，服务没有等待在途请求的机会。

这里使用带缓冲的结果 channel，退出前等待 Serve goroutine 返回，避免产生遗留发送者。当前未设置统一 WriteTimeout；未来流式接口的截止时间要随运行协议设计，不能简单套用一个很短的写超时。

### 5.4 模型流：拿到 reader 还没有完成请求

读 `internal/model/client.go` 和对应测试。

1. 校验模型名、凭据、接口根地址和整体超时。
2. 创建有截止时间的 context，用于整次流式调用。
3. 创建真实 Eino OpenAI 兼容适配器，传入 schema.UserMessage。
4. Stream 返回 reader 后逐次 Recv；正常 EOF 才代表消费结束。
5. Close reader 并取消 context，释放本次资源。

本课覆盖：请求开始失败、流中途错误、首块之后超时、预先取消、空文本、输出目标错误与短写。

错误分类区分调用取消、截止时间、供应商失败和空响应。供应商原始错误体可能包含内部信息，当前不默认透出；详细诊断与脱敏 Trace 后续补充。

当前仅消费文本，不处理工具调用、思考字段、多模态或用量汇总；未加入重试、熔断与路由。模型 SDK 支持哪些字段不等于应用已经完成相应功能。

## 6. 你现在按顺序操作

以下操作均在 `/Users/admin/workspace/ragent-go` 执行。

### 步骤 A：确认本课实际工具链

```sh
cd /path/to/ragent-go
go version
```

预期当前机器显示 `go version go1.27.3 darwin/arm64`。不需要先修改 shell 配置。

### 步骤 B：运行服务

```sh
make run
```

看到 JSON 日志 `server started` 后，在另一个终端执行：

```sh
curl -i http://127.0.0.1:8080/healthz
```

预期状态 200，响应头有 X-Request-ID，正文为 `{"status":"ok"}`。回到服务终端按 Ctrl+C，预期日志为 `server stopped gracefully`。

如果端口占用，不要结束别人的进程，改一个端口：

```sh
RAGENT_HTTP_ADDR=127.0.0.1:18080 make run
```

### 步骤 C：亲眼观察启动前校验

```sh
RAGENT_HTTP_ADDR=invalid make run
```

预期非零退出，错误包含 RAGENT_HTTP_ADDR，服务没有成功开始监听。

### 步骤 D：运行自动化验证

```sh
make test
make vet
make verify-modules
make smoke
```

`make test` 开启 race detector、45 秒测试超时并生成 coverage.out。Makefile 将缓存固定到项目内。第一次构建可能较慢，不代表运行模型较慢。

`make smoke` 使用 Python 3 标准库，实际启动 server 和 modelcheck 两个二进制；模型侧访问本机临时模拟服务，不访问外部模型。成功显示两条 PASS。

查看覆盖率时沿用同一个模块缓存，避免工具隐式尝试联网：

```sh
GOTOOLCHAIN=local GOMODCACHE="$PWD/.cache/go-mod" GOCACHE="$PWD/.cache/go-build" go tool cover -func=coverage.out
```

### 步骤 E：真实供应商联调（正常调用已验证）

编辑项目根目录 `.env` 中的配置，再运行 `make modelcheck`；已有系统环境变量会覆盖文件值：

- RAGENT_MODEL_BASE_URL：兼容接口根路径，不填完整 chat/completions 地址。
- RAGENT_MODEL_NAME：实际模型标识。
- RAGENT_MODEL_API_KEY：只填入本机被忽略的 `.env` 或安全运行环境，不写进模板、笔记、仓库或命令历史。
- RAGENT_MODEL_TIMEOUT：可选，默认 20s。
- RAGENT_MODEL_PROMPT：可选，默认一句 Agent 概念问题。

2026-10-28 已按用户提供的配置执行一次真实调用，命令退出码为 0，返回了正常的 Agent 概念回答。未读取或记录密钥、供应商地址与其他私人配置。工具输出按批次呈现，未测量逐块间隔与首包延迟；真实取消、错误配置等场景尚未额外调用。

## 7. 本次测试与验收结果

2026-10-28 已执行：

| 验收项 | 实际结果 |
|---|---|
| 测试先行 | 先确认 Load/NewHandler/Serve 等缺少实现的预期失败，再实现；模型与命令同样先写测试 |
| 单元/集成测试 | 六个 Go 包通过 `go test -race -timeout=45s` |
| 整体语句覆盖率 | Gin 迁移后为 87.2%（初版 89.7%，加入 .env 后 86.6%）；包含未被单测覆盖的 CLI main/signal 包装；不将进程冒烟计入此百分比 |
| 配置包 | 100%，包括 .env 加载与优先级、缺失/异常文件、空值覆盖与错误不泄露 |
| HTTP 包 | 100% |
| 生命周期包 | 94.1% |
| 模型包 | 96.7% |
| 入口装配 | runServer 与 runCheck 均 100%；main 的退出分支没有全部单测覆盖 |
| 静态检查 | `go vet ./...` 通过 |
| 依赖完整性 | `go mod verify` 返回 all modules verified；未执行独立漏洞扫描，不等同于无漏洞保证 |
| 构建 | server、modelcheck 二进制均构建成功 |
| 进程级冒烟 | 健康检查、请求标识、配置错误、SIGTERM 退出、.env 驱动两个入口、环境覆盖及异常文件脱敏均通过；Eino 仅调用本地模拟供应商 |
| 真实供应商 | 正常调用成功，退出码 0；真实异常/取消和耗时指标未验证 |
| 容器/数据库 | 未验证 |

配置方式更新（2026-10-28）：按用户要求增加 `.env`，以 godotenv 解析成映射，不污染进程环境。先补预期失败测试再实现；全量测试、vet、依赖校验和进程冒烟通过。冒烟使用临时目录中的 `.env`，不读取本机真实模型配置。

本机安装记录：尝试 Homebrew 安装时下载停滞，已取消；改将此前校验通过的官方工具链迁入公共用户安装目录，创建 go/gofmt 符号链接。不依赖未完成的 Homebrew 安装，项目内已无 `.tools/go`。

构建缓存排障记录：一次覆盖率子命令遗漏项目 GOMODCACHE，内部 go list 等待默认模块代理。停止该命令后，统一缓存路径并使用 GOPROXY=off，覆盖率、vet 与模块校验正常结束。检查命令与构建命令也必须共享环境约定。

覆盖率反映本课代码测试范围，不是整个平台质量指标。Go 测试以当前机器为证据，其他系统/架构仍需验证。

## 8. 复盘题与小练习

请结合代码，用自己的话解释：

1. main、runServer、Serve 分别负责什么？为什么分开？
2. 为什么模型 reader 创建成功后，仍必须处理消费过程中出现的错误？
3. Shutdown 超时时，Close 与 context 取消分别产生什么效果？
4. 为什么健康检查正常，仍不能证明模型服务可用？
5. 为什么这次 Eino 编译通过，不代表 MCP、审批恢复和所有供应商协议都已经兼容？

小练习：

- 修改 `.env` 的端口并运行；再用临时环境变量覆盖端口，观察优先级。

- 将监听端口改为 18080，启动、访问健康检查并正常退出。
- 运行配置错误示例，说明为什么没有启动成功。
- 阅读模型“流中途错误”测试，说明为什么保留 partial 文本但仍返回错误，前端未来应该怎样显示这一状态。

练习目标是能解释观察到的行为，而不仅是执行命令。

## 9. 本课产物与下一步

实际新增：两个命令入口、四个内部包及测试、依赖锁定文件、Makefile、配置说明、README 与进程冒烟脚本。

第一课程序部分已通过本地验收，真实模型正常调用已验证。下一步继续学员复盘并进入第二课工具型 Agent，再按小节推进：Run/事件契约、模型调用与应用请求的关联、基础身份作用域；ADK/MCP/检查点的兼容原型按使用顺序展开。暂不一次生成 RAG 或 Agent 全部代码。

.env 解析器依据：[godotenv v1.5.1 官方说明](https://github.com/joho/godotenv/blob/v1.5.1/README.md)，具体读取行为见本项目 Environment 及其测试。


### 真实联调兼容性记录

Go 1.27.3 下 Sonic 输出兼容性提示：其优化 AST 路径不支持当前 Go 版本，回退到标准 encoding/json。此次请求成功；本轮不据此宣称性能等同于 Sonic 优化路径。后续性能基线前需要选择依赖明确支持的 Go/编码器组合并回归，不将单次成功理解为完整兼容认证。


## 10. HTTP 层迁移到 Gin（2026-10-28）

### 目标与职责

用户确认使用 Gin 建设后续业务接口。本次将路由与中间件迁到 Gin v1.12.0；配置、模型逻辑与服务生命周期保持各自职责。

```text
main：读取 .env → 配置校验 → 日志/监听器装配
                       ↓
http.Server：连接与请求生命周期
                       ↓
Gin Engine：路由与中间件
                       ↓
请求标识与日志 → panic 恢复 → healthz Handler
```

Gin Engine 实现 http.Handler，因此可以交给已有 bootstrap.Serve；不需要调用 router.Run() 再启动一套服务。标准库 http.Server 与 Gin 不是互斥选择。

### 代码阅读顺序

1. `cmd/server/main.go`：进程入口设置 Gin release 模式；runServer 继续按原方式装配。
2. `internal/handler/handler.go`：用 gin.New 显式注册中间件和 GET/HEAD 路由，返回值仍是 http.Handler。
3. `internal/bootstrap/server.go`：没有改动，继续负责停止接收请求、限时等待和强制关闭。
4. `internal/handler/gin_test.go`：观察框架迁移怎样用行为测试控制差异。

`*gin.Context` 是路由和中间件上下文，与标准库 context.Context 职责不同。同步调用下游时使用 c.Request.Context()；不得把会被复用的 *gin.Context 直接留给后台任务。后续 Run 的后台执行生命周期需要单独设计。

### 为什么不直接使用 gin.Default()

使用 gin.New 显式选择日志和恢复策略，沿用项目 slog JSON 日志，避免重复日志。恢复策略使用 Gin 的 CustomRecoveryWithWriter，但不让默认恢复日志打印请求或原始 panic 内容；应用只记录请求标识与响应是否已提交。

进程入口设为 release 模式以避免框架调试输出，业务日志级别仍来自 .env 的 RAGENT_LOG_LEVEL；构造函数不修改 Gin 全局模式。

### 协议与安全细节

| 项目 | 本项目行为 | 验证结果 |
|---|---|---|
| GET /healthz | 200、application/json、status=ok | 通过 |
| HEAD /healthz | 200、无响应正文 | 通过 |
| POST /healthz | 405，Allow 包含 GET 和 HEAD | 通过 |
| /healthz/ | 不自动跳转，返回 404 | 通过 |
| 请求日志 | 服务端生成请求 ID，记录方法、路径、最终状态和耗时；不记录查询参数/请求头/正文 | 通过 |
| panic | 未提交响应时返回脱敏 JSON 500；不泄露 panic 值或请求秘密 | 通过 |
| 部分响应后 panic | 不覆盖已发送状态，不追加伪造的完整错误正文；记录异常 | 通过 |
| 代理信任 | SetTrustedProxies(nil)，忽略不可信的 X-Forwarded-For | 通过 |
| 流式 writer | 通过 Gin writer 保留 Flush 能力，未额外包裹不支持流式的 writer | 测试通过；真实业务 SSE 尚未实现 |

未来有反向代理时，需要明确可信代理地址；当前未实现登录鉴权、业务限流或完整 SSE 协议。已提交响应后的 panic 无法改成 HTTP 500，不应以已有 2xx 状态单独判断业务任务成功。

JSON 响应仍显式使用 Go 标准库 encoding/json，不因引入 Gin 就切换 JSON 实现。Gin 增加了传递依赖，go.mod/go.sum 已更新；没有因为依赖中出现某个包就启用其全部功能。

### 测试过程与结果

先写迁移协议、Gin 集成、恢复脱敏和代理信任测试，确认原实现失败：缺少最终状态日志、HEAD recorder 有正文、尚无 Gin 集成。再实现迁移，针对性测试通过后运行全量验证。

已执行：make test、make vet、make smoke、make verify-modules（使用项目模块缓存和已有构建缓存）。六个 Go 包竞态测试通过，HTTP 包覆盖率 100%，全工程语句覆盖率 87.2%；真实进程验证 GET/HEAD/405、JSON 请求日志与 SIGTERM 退出通过。模型只访问本地测试服务，无额外真实模型调用。

### 你现在可以复现

```sh
cd /path/to/ragent-go
make run
```

另开终端（端口按 .env 调整）：

```sh
curl -i http://127.0.0.1:8080/healthz
curl -I http://127.0.0.1:8080/healthz
curl -i -X POST http://127.0.0.1:8080/healthz
```

分别观察 200、200 无正文、405，以及对应带 status 的 JSON 日志。Ctrl+C 后观察正常退出。

复盘：

1. 为什么用了 Gin，仍然保留 http.Server？
2. c.Next() 前后的代码分别何时执行？恢复中间件为何在请求日志中间件内层？
3. 为什么响应已发送后，不能再通过写一个 500 把错误“修好”？
4. 为什么代理头不能直接当作可信用户来源？

官方依据（核查日期 2026-10-28）：[Gin v1.12.0 文档源码](https://github.com/gin-gonic/gin/blob/v1.12.0/docs/doc.md)、[Engine 源码](https://github.com/gin-gonic/gin/blob/v1.12.0/gin.go)、[Recovery 源码](https://github.com/gin-gonic/gin/blob/v1.12.0/recovery.go)。具体行为以本项目锁定版本及测试为准。


## 11. 第一课 Git 提交

2026-10-28，本地 main 分支提交 `c607fe7`：`feat: bootstrap lesson one with Gin and Eino`。提交 23 个文件，包含第一课代码、测试、依赖锁定和运行说明。提交前测试、vet、进程冒烟与依赖校验均通过，暂存差异检查通过；提交后代码仓库工作区干净。真实 .env、.idea、缓存、覆盖率和构建产物均未纳入 Git。该次提交时课程文档保存在独立目录；现已迁入项目 docs/ 并随 Git 版本管理。
