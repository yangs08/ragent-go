# Ragent Go 学习入口

首要目标是学习 Agent 和 Eino，参考 Ragent 源码，通过 Go 企业项目验证理解。正式文档只维护在项目 docs/；Obsidian 目录保留访问入口。

## 从这里开始

- [学习路线](ROADMAP.md)：核心专题与学习方法；当前结合 A01/A03/A04 落实多轮和增量执行。
- [当前架构](ARCHITECTURE.md)：真实目录、装配、身份、事务和执行边界。
- [项目设计与能力地图](PROJECT.md)：长期项目范围；交付阶段不等于课程顺序。
- [接口与 SSE 契约](api/README.md)：区分已实现接口与计划接口。
- [配置说明](../configs/README.md)：YAML、环境覆盖、密钥和 Boot。

## 当前专题实践

- [A01 多轮消息与增量输出](lessons/A01-agent-conversation-and-streaming.md)：已接入真实 Agent 和 CLI；完整消息历史、部分失败、用户上下文和进程取消验证见课文。

## 已有课程记录

以下保存当时的学习内容、运行结果与限制，不表示学员已全部掌握。历史路径和示例以当前架构及代码为准。

| 记录 | 已有基础与待验证内容 |
|---|---|
| [01 工程与模型调用](lessons/01-project-bootstrap.md) | 本地验证和真实文本调用有记录；真实异常与取消待补 |
| [02 工具型 Agent](lessons/02-agent-and-tools.md) | ADK / 工具 / CLI 本地验证有记录；真实供应商工具联调待补 |
| [03 接口与运行契约](lessons/03-api-and-run-contract.md) | 契约已有，聊天 HTTP / SSE 尚未实现 |
| [04 PostgreSQL 与迁移](lessons/04-postgresql-and-migrations.md) | 数据库、迁移及事务实验有记录 |
| [05 登录与用户上下文](lessons/05-login-and-user-context.md) | 认证与用户隔离验证有记录 |

过期的架构整改补充课文已合并到当前架构说明，不再维护多份相互替代的规范。

## 学习记录约定

1. 新专题沿用 A 编号，历史 01–05 不改号；开课时才创建实际课程文件。
2. 使用 [课程模板](lessons/TEMPLATE.md)，先写核心问题和预测，再记录源码、实验、结果与复盘。
3. 分别记录代码已有、实验已执行、学员能解释/修改；没有证据的不标完成。
4. 框架接口记录实际依赖版本和官方依据；源码记录参考提交与文件位置。
5. 命令写清执行目录，结果去除凭据；不把未执行的计划写成成功。
6. 复杂逻辑注释解释原因和边界；工程补充服务当前专题。
7. 当前架构变更回写 ARCHITECTURE.md，长期范围变更回写 PROJECT.md。
