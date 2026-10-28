# Ragent Go

基于 Go + Eino 的 Agentic RAG 学习项目。当前为第一课工程：HTTP 启动底座与独立模型验证命令。

课程文档：`/Users/admin/workspace/docs/studyroom/ragent-go/lessons/01-project-bootstrap.md`。

## 当前能力

- 配置加载与启动前校验，默认只监听 `127.0.0.1:8080`。
- Gin v1.12.0 路由与中间件，`/healthz`、JSON 日志与服务端生成的请求标识。
- 请求完成状态记录、脱敏 panic 恢复与显式代理信任策略。
- SIGINT/SIGTERM 后限时等待在途 HTTP 请求，超时强制关闭。
- Eino 真实适配器的独立模型验证命令，包含流式消费、整体超时和取消。
- 本地模拟供应商集成测试，不需要真实 API Key。

HTTP 服务目前仅提供健康检查。没有认证、聊天 API、持久会话或 Agent，尚不能部署为企业平台；后续按课程添加。

## 工具链与依赖

本机 Go 1.27.3 已安装到用户级公共目录 `/Users/admin/.local/share/go/1.27.3`，命令入口为 `/Users/admin/.local/bin/go` 与 `gofmt`，所有项目可直接调用。Makefile 默认使用 PATH 中的 `go`，不再依赖项目 `.tools`。其他机器可安装兼容 Go，或使用 `make GO=/absolute/path/to/go ...`。HTTP 框架 Gin v1.12.0；核心 Eino v0.9.22、模型扩展 v0.1.13，完整依赖记录在 go.mod/go.sum。

Makefile 的构建与模块缓存放在项目 `.cache/`，不修改全局 Go 配置。首次下载使用可达的模块代理；例如本机验证使用 `GOPROXY=https://goproxy.cn`。不关闭校验数据库。

## 运行

```sh
cd /Users/admin/workspace/ragent-go
make run
```

另一个终端：

```sh
curl -i http://127.0.0.1:8080/healthz
```

按 Ctrl+C 正常退出。启动时读取当前工作目录的 `.env`；本地开发从项目根目录运行。优先级：系统环境变量 > `.env` > 程序默认值。文件不存在时可使用环境变量和默认值，格式/读取错误会在启动时报错。配置说明见 configs/README.md。

已创建本机 `.env`（不入版本控制），默认可启动 HTTP 服务；编辑该文件即可调整端口和模型配置。`.env.example` 为无凭据模板，新检出项目且尚无 `.env` 时再复制模板。

## 检查

```sh
make test
make vet
make verify-modules
make smoke
```

`make smoke` 需要 Python 3 标准库，启动本地临时端口和本地模拟供应商，验证两个已构建进程；不访问付费模型。

## 真实模型联调（尚未执行）

在项目根目录 `.env` 填写 `RAGENT_MODEL_BASE_URL`、`RAGENT_MODEL_NAME`、`RAGENT_MODEL_API_KEY` 后运行 `make modelcheck`。可选 `RAGENT_MODEL_TIMEOUT`（默认 20s）和 `RAGENT_MODEL_PROMPT`。Base URL 使用供应商指定的兼容接口根路径，例如以 `/v1` 结尾的地址；不要填完整 `/chat/completions`。

真实凭据只保存在被 Git 忽略的本机 `.env` 或安全运行环境中，不写入示例、文档或命令历史。该命令会实际调用配置的服务，可能产生用量费用，本轮未执行。

当前模型验证只消费文本；工具调用、思考字段、多模态、用量汇总与供应商差异属于后续实现范围。

## HTTP 框架边界

Gin 负责路由与中间件，作为 http.Handler 交给现有 http.Server。端口监听、超时和优雅退出仍由 internal/bootstrap 管理，不调用 router.Run() 启动另一个服务。

进程入口将 Gin 设置为 release 模式，应用日志级别继续由 RAGENT_LOG_LEVEL 控制。使用 gin.New() 显式装配中间件，默认不信任代理头；需要反向代理时再显式配置可信代理。

GET/HEAD /healthz 返回 200；HEAD 无正文；POST 返回 405 和 Allow；/healthz/ 返回 404，不自动重定向。JSON 响应继续使用 encoding/json，没有启用额外 JSON 替换构建参数。

panic 在响应尚未提交时返回脱敏 500；响应已提交则保留已发送状态与内容、记录恢复事件，不能把部分输出伪装成完整成功。panic 的测试路径只存在于测试，不在服务开放。
