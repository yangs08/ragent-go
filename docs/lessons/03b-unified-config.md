# 第 03 课补充实践：统一应用配置

## 目标与 Ragent 对照

配置来源、默认值、解析和合法性校验由 internal/config 统一管理。Ragent 使用 Spring 将配置段绑定到 AgentProperties 等类型，再注入组件。Go 版本使用启动层加载与构造函数传参：同样集中管理、按模块分组，不复制 Spring 容器。

原有问题是 config.Load、model.LoadSettings、agent.LoadOptions 分散解析环境配置。现在只有 config.Load 对调用方公开加载入口。

## 当前结构

```text
internal/config/
  environment.go   .env 与系统环境变量合并
  config.go        Config、ServerConfig 与统一 Load
  model.go         ModelConfig、模型配置解析与校验
  agent.go         AgentConfig、Agent 配置解析与校验
```

```go
type Config struct {
    Server ServerConfig
    Model  ModelConfig
    Agent  AgentConfig
}
```

Server：HTTPAddr、ShutdownTimeout、LogLevel。
Model：BaseURL、Model、APIKey、Timeout。
Agent：MaxIterations、MaxToolCalls、Timeout。

配置类型、解析函数和校验归 config 所有。Agent 不再有 Options 或 LoadOptions；模型模块不再有 Settings 或 LoadSettings。内部 loadServer/loadModel/loadAgent 是分组实现细节，不是独立的启动入口。

## 启动与使用

```go
getenv, err := config.Environment(".env", os.LookupEnv)
// 检查 err
cfg, err := config.Load(getenv)
// 检查 err
client, err := model.NewClient(ctx, cfg.Model)
// 检查 err
orderAgent, err := agent.NewOrderAgent(client, store, principal, cfg.Agent)
```

上述省略错误分支只为展示调用关系，实际代码必须处理错误。配置在进程入口读取，组件接收自己需要的配置值；组件不会重新查询环境。不要打印整个 cfg，Model 中含密钥。

统一配置不等于可变全局变量。当前各组件通过值接收配置快照，测试可以独立构造配置，不修改进程全局状态。未来配置热更新若有实际需求，应单独设计快照替换与连接重建规则。

## 默认与校验行为

.env 字段及优先级保持不变：系统环境变量优先于文件，最后应用字段默认值。显式空的系统环境变量继续覆盖文件值。

当前 server 只有健康检查，允许模型 URL、名称、密钥全部未配置；如果任一非空，则模型配置必须完整合法。仅配置模型超时也必须是合法时长。modelcheck/agentcheck 创建模型客户端时调用 ModelConfig.Validate，因此不能无凭据执行。

统一 Load 会校验所有配置段：即使只运行 modelcheck，非法 Agent 或 Server 配置也会阻止启动。这是统一应用配置的明确行为。启用真实聊天接口后，需要在应用装配阶段将完整模型配置作为服务启动前提。

构造函数仍会调用 config 中的 Validate，避免测试或其他调用方直接构造无效配置后绕过校验；校验规则只有一份。Agent 的迭代次数、工具预算和整轮超时属于 Agent 配置，不代表每次运行都要重新读取环境。

## 验证与练习

运行 make test、make vet、make smoke、make verify-modules。使用本地模拟模型协议验证，不调用真实付费模型。

测试覆盖统一加载、自定义值、非法数字/时长/URL、密钥错误脱敏、缺配置拒绝模型执行，以及既有 .env 优先级、模型流式、Agent 工具调用与预算行为。

阅读 cmd/agentcheck/main.go：找出配置加载、客户端构造、执行和输出四个步骤。思考：配置为什么由启动层加载？为什么构造函数仍需校验？为什么不将当前用户身份放进 Config？为什么 cfg.Agent.Timeout 与 cfg.Model.Timeout 需要分别管理？

本课只调整配置体系。订单示例、用户身份生命周期与聊天应用服务仍按 ARCHITECTURE.md 继续整改。
