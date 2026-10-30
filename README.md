# Ragent Go

基于 **Go、Gin 和 Eino** 的 Agentic RAG 工程实践项目。参考 [Ragent](https://github.com/nageoffer/ragent) 的能力与设计，面向 Go 开发者，通过逐课实现、源码导读、测试和故障实验，逐步建设完整的企业级 AI 应用平台。

项目按 [40 节主课](docs/ROADMAP.md) 推进，目标覆盖知识入库、混合检索、RAG、Agent 工具调用、MCP、Skills、会话与长期记忆、人工确认、权限隔离、可观测性和部署验收。**当前完成前两课的代码与本地验证，仍处于持续开发阶段。**

这是独立的 Go 学习与实现项目，与参考项目不存在官方隶属关系。已实现能力和后续计划分别记录在下方及课程路线中。

课程入口：[学习目录](docs/README.md)、[40 节主课路线](docs/ROADMAP.md)、[项目设计](docs/PROJECT.md)。

已开课程：[第 01 课：工程底座](docs/lessons/01-project-bootstrap.md)、[第 02 课：工具型 Agent](docs/lessons/02-agent-and-tools.md)。文档实际保存在项目 docs/，与代码共同提交，克隆即可阅读。

## 当前能力

- 配置加载与启动前校验，默认只监听 `127.0.0.1:8080`。
- Gin v1.12.0 路由与中间件，`/healthz`、JSON 日志与服务端生成的请求标识。
- 请求完成状态记录、脱敏 panic 恢复与显式代理信任策略。
- SIGINT/SIGTERM 后限时等待在途 HTTP 请求，超时强制关闭。
- Eino 真实适配器的独立模型验证命令，包含流式消费、整体超时和取消。
- 本地模拟供应商集成测试，不需要真实 API Key。

HTTP 服务目前仅提供健康检查，订单 Agent 通过独立 CLI 演示。认证已在第五课接入；尚无聊天 API、持久聊天会话和管理后台，后续按课程添加。

## 工具链与依赖

当前验证环境为 Go 1.27.3（macOS arm64），`go.mod` 记录工具链要求。Makefile 默认使用 PATH 中的 `go`，也可用 `make GO=/absolute/path/to/go ...` 指定。HTTP 框架 Gin v1.12.0；Eino 核心 v0.9.22、模型扩展 v0.1.13，完整依赖记录在 go.mod/go.sum。

当前 Sonic v1.15.0 在 Go 1.27 下会提示回退到标准库 JSON；现有测试与模型文本调用通过，尚未验证优化路径性能。我们自己的业务 JSON 默认使用 `encoding/json`。

Makefile 的构建与模块缓存放在项目 `.cache/`，不修改全局 Go 配置。首次下载使用可达的模块代理；例如本机验证使用 `GOPROXY=https://goproxy.cn`。不关闭校验数据库。

## 运行

```sh
git clone git@github.com:yangs08/ragent-go.git
cd ragent-go
cp .env.example .env
make run
```

另一个终端：

```sh
curl -i http://127.0.0.1:8080/healthz
```

按 Ctrl+C 正常退出。启动时读取当前工作目录的 `.env`；本地开发从项目根目录运行。优先级：系统环境变量 > `.env` > 程序默认值。文件不存在时可使用环境变量和默认值，格式/读取错误会在启动时报错。配置说明见 configs/README.md。

首次克隆时复制 `.env.example` 到 `.env`，默认配置即可启动健康检查。已有 `.env` 时请保留，不要覆盖。模型相关配置留空，使用模型命令前再填写自己的服务信息。真实 `.env` 不入版本控制。

## 检查

```sh
make test
make vet
make verify-modules
make smoke
```

`make smoke` 需要 Python 3 标准库，启动本地临时端口和本地模拟供应商，验证三个已构建进程；不访问付费模型。

## 真实模型联调

在项目根目录 `.env` 填写 `RAGENT_MODEL_BASE_URL`、`RAGENT_MODEL_NAME`、`RAGENT_MODEL_API_KEY` 后运行 `make modelcheck`。可选 `RAGENT_MODEL_TIMEOUT`（默认 20s），问题通过命令 `-prompt` 参数传入。Base URL 使用供应商指定的兼容接口根路径，例如以 `/v1` 结尾的地址；不要填完整 `/chat/completions`。

真实凭据只保存在被 Git 忽略的本机 `.env` 或安全运行环境中，不写入示例、文档或命令历史。该命令会实际调用配置的服务，可能产生用量费用。开发阶段已验证一次真实文本调用；自动化测试和冒烟只使用本地模拟服务。

`modelcheck` 只消费文本；工具循环请使用第二课的 `agentcheck`。思考字段、多模态、用量汇总与完整供应商兼容性仍属后续范围。

## HTTP 框架边界

Gin 负责路由与中间件，作为 http.Handler 交给现有 http.Server。端口监听、超时和优雅退出仍由 internal/bootstrap 管理，不调用 router.Run() 启动另一个服务。

进程入口将 Gin 设置为 release 模式，应用日志级别继续由 RAGENT_LOG_LEVEL 控制。使用 gin.New() 显式装配中间件，默认不信任代理头；需要反向代理时再显式配置可信代理。

GET/HEAD /healthz 返回 200；HEAD 无正文；POST 返回 405 和 Allow；/healthz/ 返回 404，不自动重定向。JSON 响应继续使用 encoding/json，没有启用额外 JSON 替换构建参数。

panic 在响应尚未提交时返回脱敏 500；响应已提交则保留已发送状态与内容、记录恢复事件，不能把部分输出伪装成完整成功。panic 的测试路径只存在于测试，不在服务开放。

## 第二课：只读订单 Agent

在现有 .env 配置的模型支持工具调用时执行：

```sh
make agentcheck
```

该命令会真实调用已配置服务，通常包含多次模型请求，可能产生用量；本课自动化验证只访问本地模拟服务。事件按 JSONL 输出：tool_call、tool_result、answer。当前按完整消息显示步骤，不是逐 Token 输出，也不是已经实现 HTTP SSE。

开发用户由 CLI 注入为 alice：A100=shipped，A101=delivered；B200 和 C300 属于其他用户，对当前身份均返回 not_found。没有真实写操作、真实业务订单或生产登录认证。

可选配置：RAGENT_AGENT_MAX_ITERATIONS（默认 4）、RAGENT_AGENT_MAX_TOOL_CALLS（默认 3）、RAGENT_AGENT_TIMEOUT（默认 30s）。未设置时使用默认值，无需改动现有 .env。单次模型请求仍受 RAGENT_MODEL_TIMEOUT 约束。

执行器支持工具调用与无工具的澄清回答；订单提示词要求具体状态依据查询结果，程序目前不保证自然语言与结果完全一致，答案忠实性与效果评估会继续建设。完整讲解见 [第二课文档](docs/lessons/02-agent-and-tools.md)。

当前职责与综合整改说明见 [应用架构基线](docs/ARCHITECTURE.md) 和 [第三课补充实践](docs/lessons/03c-architecture-refactor.md)。

## 项目结构

```text
cmd/           server、modelcheck、agentcheck 命令入口
internal/      HTTP、配置、模型、Agent 与受控工具实现
configs/       配置字段说明
scripts/       本地进程冒烟验证
docs/          项目设计、40 节路线、逐课讲解与验收记录
```

## 参考与学习方式

- [Ragent](https://github.com/nageoffer/ragent)：业务能力与 Java 源码参考。
- [Eino](https://github.com/cloudwego/eino)：Go AI 组件与 Agent 执行框架。
- 每课先明确业务与边界，再实现和测试；实际结果、限制和提交记录同步写入 docs/。
- 当前没有完整生产认证、持久会话、知识索引或管理后台，上线能力以各阶段验收证据为准。

## 第四课：PostgreSQL + GORM

本地数据库与其他项目隔离，默认 127.0.0.1:15432。设置 .env 中 RAGENT_DB_ENABLED=true 与独立随机密码后执行：

```sh
make db-up
make migrate
make migrate-status
make dbcheck
```

使用独立 SQL 迁移，不在应用启动时 AutoMigrate。当前本机连接、重复迁移、事务回滚与数据库启用后的服务退出已验证。数据库健康不是 /healthz 的检查内容，登录已在第五课接入，聊天会话仍待实现。详见 [第四课](docs/lessons/04-postgresql-and-migrations.md)。

## 第五课：登录认证

数据库开启并执行 make migrate 后，可用 make user-create USERNAME=alice 创建账号，终端输入密码不回显。make run 提供 POST /auth/login、POST /auth/logout、GET /user/me；Authorization 直接传登录返回的 token。登录结果沿用 HTTP 200 + 业务 code，检查 code 是否为字符串 "0"。

可执行 make auth-smoke 验证本项目专属容器中的真实认证链路，会自动清理随机测试账号。角色与用户管理后台尚未实现，细节见 [第五课](docs/lessons/05-login-and-user-context.md)。

## 配置与启动约定

普通配置维护在 configs/*.yaml；.env 保存密钥与本机覆盖。优先级：系统环境变量 > .env > YAML > 程序默认值。已有 .env 未自动迁移，同名配置仍会覆盖 YAML。

bootstrap.Boot 统一初始化 bootstrap.Config、bootstrap.DB、bootstrap.Logger，业务响应复用 define.Result。详见 [配置说明](configs/README.md) 与 [整改课文](docs/lessons/05a-boot-config-response.md)。修改完成后等待用户确认，不自动 commit。

当前配置已改为 define/config.go 的结构体标签自动映射，公共启动变量直接放 bootstrap/boot.go；已移除 global 目录。详见 05a 整改课文。

当前入口使用 bootstrap.Boot(ctx)，日志统一 stderr。model 存放数据库实体，AI 客户端位于 service/model.go，共享函数位于 internal/common/functions.go。Diagnostics 与配置 Validate 已移除，联调问题使用 -prompt 参数。
