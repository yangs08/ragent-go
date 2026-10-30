# 第 04 课：PostgreSQL、GORM 与数据库迁移

日期：2026-10-29。状态：本机独立 PostgreSQL 验证完成，尚未实现登录与会话业务。

## 目标与技术选择

本课建立后续聊天服务真正需要的数据底座：连接池、启动检查、独立迁移、事务与退出释放。按学员选择使用 GORM 进行 CRUD；数据库结构用独立 SQL 文件和 Goose 管理，不使用 GORM AutoMigrate，不自写迁移框架。

锁定版本：GORM v1.31.2、gorm PostgreSQL 驱动 v1.6.3、Goose v3.29.0，底层 pgx v5.11.0。由 Go 模块解析核实版本并编译验证；go.mod/go.sum 是依赖依据。

## 先对照 Ragent

Ragent 的 application.yaml 配置 PostgreSQL 数据源，UserDO 使用 MyBatis-Plus 映射 t_user，resources/database/schema_pg.sql 与 upgrades 下的 SQL 管理结构。Go 版对应为 config.Database、GORM 和版本化迁移，不逐类复制 Java 数据访问框架。

第一张 users 表是 Go 版下一课认证的最小基础，包含 id、唯一 username、password_hash、created_at/updated_at。它不是导入原 Ragent t_user 的完整结构，也不包含角色、头像、软删除或租户字段。本课不生成真实登录账号，登录业务及密码散列在第 05 课实现。

## 当前文件与调用关系

```text
internal/bootstrap/config.go  数据库配置与校验
internal/database/postgres.go GORM 初始化与连接池配置
migrations/00001_create_users.sql 第一份结构迁移
migrations/embed.go          将 SQL 嵌入迁移命令
cmd/migrate/main.go          显式 status/up 命令
cmd/dbcheck/main.go          实际连接与事务回滚验证
cmd/server/main.go          启用数据库时连接、启动检查、退出关闭
 deploy/compose.yaml        本项目独立的 PostgreSQL
```

入口读取配置 → database.Open → GORM *gorm.DB → 底层 *sql.DB 连接池。后续 Service 接收 *gorm.DB，不在每个请求中重新连接，不创建只有转发作用的 BaseRepository。

## GORM 与连接池分别做什么

*gorm.DB 表达查询与事务；底层 database/sql 维护真实连接。进程启动时建立一个池，服务退出时关闭。GORM 的 WithContext(ctx) 让查询遵循本次请求的取消/期限；不要把某个用户请求的 context 永久绑定到共享 DB。

当前默认最大连接 10、空闲连接 5、连接最长寿命 30m、最大空闲时间 5m。多实例部署时总连接数约为实例数乘以每实例池上限，不能只看单实例。当前值是开发起点，不是生产容量结论。

Open 在指定期限内 Ping，连接失败会在监听 HTTP 前报错；初始化失败清理连接。SQL 日志默认关闭，避免打印 SQL 参数与敏感数据；对外不回显连接串或密码。远程连接要求 verify-full，本机回环可使用 disable。

## 本地数据库

本机 5432 已被其他项目使用，所以创建独立 Compose 项目 ragent-go-study，监听 127.0.0.1:15432，数据卷 ragent-go-study_postgres_data。没有修改已有数据库容器。使用已有 pgvector/pgvector:pg16 镜像，但本课没有启用 vector 扩展，向量索引仍在后续课程。

本机已向真实 .env 追加独立数据库配置与随机密码，保留原有模型配置。模板中的 RAGENT_DB_ENABLED=false；本机本课环境启用 true。其他机器首次配置时启用该开关并设置自己的随机密码，不要复制或提交真实 .env。

在项目根目录执行：

```sh
make db-up
make migrate
make migrate-status
make dbcheck
make run
```

- db-up：创建/启动专属数据库并等待健康状态。
- migrate：显式执行待应用迁移，已有版本不重复执行。
- migrate-status：查看当前版本，当前应为 1。
- dbcheck：连接、在事务内插入/读取随机测试记录、主动回滚，并确认没有残留。
- run：启用数据库时先检查连接，再监听 HTTP。仍只有 /healthz。

停止数据库可执行 make db-stop，保留数据卷。不要随意删除卷；修改 .env 密码不会自动修改已经初始化的数据库角色密码。

## 为什么迁移不放在 HTTP 启动里

应用上线与结构变更有不同的职责和失败处理。当前迁移通过独立命令执行，并使用 PostgreSQL session advisory lock 防止迁移并发执行；SQL 的 Up 默认事务执行。迁移历史由 Goose 维护，嵌入二进制后不依赖运行时查找 SQL 文件。

发布顺序：备份与审查 SQL → 独立迁移任务 → 启动应用。服务进程不执行 AutoMigrate。已有版本不要改内容，用新编号追加变更；Goose 本身不是迁移内容的 checksum 审计工具。

SQL 文件包含 Down 便于理解反向操作，但项目没有暴露 down 命令，绝不把 DROP TABLE 当成默认故障恢复方式。本课无业务数据，不能把这点推论为未来迁移都能安全回滚。

status 依赖 Goose 的版本表，初次状态访问可能初始化版本元数据；它不保证绝对只读。当前服务启动只验证连接，不验证所有应用表版本；未来有业务查询后需补上结构兼容检查。

## 事务实验

查看 cmd/dbcheck/main.go：GORM Transaction 回调返回 nil 表示提交，返回 error 表示回滚。本课使用内部哨兵错误触发回滚，事务内查询可看到刚写入记录，结束后从池外查询必须为零。

事务内部必须使用回调提供的 tx；混用共享 db 会让查询离开当前事务。这个实验验证的是实际 PostgreSQL，不用内存数据库代替，但它不等于完整并发或所有事务隔离级别验证。

## 本次实际结果

- 容器健康；首次迁移 count=1、version=1；重复迁移 count=0。
- dbcheck 连接、事务内读取、回滚验证通过；users 行数为 0。
- 使用错误密码时退出失败，输出未包含测试密码。
- 启用数据库的真实服务进程成功响应 /healthz，并在 SIGTERM 后正常退出。
- 原有 race 回归、vet、三个命令的离线冒烟和模块校验通过；没有为简单配置与传参增加新单测。
- 新依赖带来的间接依赖升级已在本轮整体验证中检查；不代表已完成漏洞扫描。

本次验证未调用真实模型，未改现有项目数据库。当前本地数据库用户用于开发初始化，生产应区分迁移账号和最小权限运行账号；本课未配置生产账号权限、备份或连接代理。

## 阅读顺序与练习

先读 bootstrap/config.go，再读 database/postgres.go，然后看迁移 SQL、migrate 命令、dbcheck 命令，最后看 server 的连接与 defer Close。

练习：执行 make migrate-status 和 make dbcheck，解释结果；再解释为什么不能每请求 gorm.Open、为什么开启事务后必须用 tx、为什么更新密码配置不会改变持久卷中的角色、为什么 /healthz 不等于业务就绪。

下一课实现登录用户与认证：定义实际用户模型、密码散列和登录流程，再由 Handler/Service 使用数据库。会话持久化在第 06 课接入。

## 官方依据

- [GORM 数据库与连接池](https://gorm.io/docs/connecting_to_the_database.html)
- [Goose Provider、迁移与 session lock](https://pressly.github.io/goose/documentation/provider/)

后续组织调整：普通配置改为 YAML + .env 分层，入口统一 Boot，全局共享启动资源，Result 集中到 common。见 [05a](05a-boot-config-response.md)，旧初始化代码片段按历史阶段理解。

最新目录与入口见 [05a](05a-boot-config-response.md)：Boot(ctx)、model 数据实体、service/model.go，以及 -prompt 联调输入；旧代码片段是当时阶段记录。
