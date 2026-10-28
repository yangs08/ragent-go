# 配置说明

启动时读取当前工作目录的 `.env`，本地请从项目根目录执行 `make run` / `make modelcheck`。

优先级为：系统环境变量 > `.env` > 程序默认值。显式存在的空环境变量也会覆盖文件值，之后再按对应字段的默认/必填规则处理。文件不存在时继续使用系统环境变量与默认值；文件不可读或语法错误时失败，错误信息不回显内容。

使用 godotenv v1.5.1 解析为配置映射，不修改进程全局环境；支持常用引号与注释语法。`.env` 已被 Git 忽略，本地文件权限为 0600；`.env.example` 可提交且不含真实密钥。运行时不会自动展开 shell 命令，也不要使用 `source .env` 加载。

| 环境变量 | 默认值 | 含义 |
|---|---|---|
| RAGENT_HTTP_ADDR | 127.0.0.1:8080 | HTTP 监听地址，必须显式包含 host 和 1..65535 的端口 |
| RAGENT_SHUTDOWN_TIMEOUT | 5s | 停止服务时等待在途请求，范围 (0,1m] |
| RAGENT_LOG_LEVEL | INFO | slog 日志级别 |

`/healthz` 只表示 HTTP 进程可响应，未检查模型、数据库或其他依赖。不把它当成业务就绪检查。

## 独立模型验证命令

| 环境变量 | 默认值 | 含义 |
|---|---|---|
| RAGENT_MODEL_BASE_URL | 无，必填 | 兼容接口根路径；HTTPS，仅回环地址允许 HTTP |
| RAGENT_MODEL_NAME | 无，必填 | 模型标识 |
| RAGENT_MODEL_API_KEY | 无，必填 | 通过安全环境注入 |
| RAGENT_MODEL_TIMEOUT | 20s | 整次流式调用截止时间，范围 (0,5m] |
| RAGENT_MODEL_PROMPT | 请用一句话解释什么是 Agent。 | 联调输入 |

Base URL 禁止内嵌凭据、查询参数或 fragment。该命令仅用于开发联调，不在 HTTP 服务开放收费模型调用入口。

## 第二课订单 Agent

`make agentcheck` 读取同一 .env 与模型配置，不需要第二份密钥。

| 环境变量 | 默认值 | 含义 |
|---|---|---|
| RAGENT_AGENT_PROMPT | 查询 A100 订单的状态，请先调用订单工具再回答。 | 本次任务 |
| RAGENT_AGENT_MAX_ITERATIONS | 4 | 模型决策轮数上限，1..20 |
| RAGENT_AGENT_MAX_TOOL_CALLS | 3 | 实际工具执行次数预算，1..20，每次运行独立 |
| RAGENT_AGENT_TIMEOUT | 30s | 整次任务截止时间，(0,5m] |

RAGENT_MODEL_TIMEOUT 是单次模型 HTTP 请求的超时；整次任务可能多次调用模型，所以另设任务超时。示例身份固定在程序中，模型参数只接受 order_id，不能切换租户或用户。
