# TaskQ 第一阶段设计与学习开发文档

## 1. 文档目的

本文档定义 TaskQ 第一阶段的功能边界、架构、开发顺序和验收标准。第一阶段以学习完整的 Go 项目开发过程为主要目标，由学习者亲自编写代码，并通过小步实现、测试和代码审阅逐步完成。

当前版本使用 Go。未来可以在需求与外部行为稳定后使用 Rust 重构，通过同一套设计和测试对比两种语言的工程实践。

## 2. 项目定位

TaskQ 是运行在单台主机上的轻量级本地任务管理工具。用户通过 CLI 提交命令，后台 daemon 执行命令、保存任务状态并收集日志。

第一阶段只实现一个最小闭环：

```text
启动 taskqd
→ taskq run echo hello
→ taskq ps
→ taskq logs <job-id>
```

开发环境为 macOS，最终运行环境为 Linux。核心代码必须使用两者共有的 Unix 能力，Linux 专属功能留到后续阶段。

## 3. 第一阶段范围

### 3.1 功能目标

- 启动一个单用户 `taskqd` daemon。
- 通过 `taskq run <program> [args...]` 异步提交任务。
- 通过 `taskq ps` 查询当前 daemon 中的任务。
- 通过 `taskq logs <job-id>` 查看任务的合并输出。
- 记录任务从创建到结束的状态变化。
- 区分命令启动失败、成功退出和非零退出。
- daemon 中的单个任务失败不能影响其他请求。

### 3.2 明确不做

第一阶段不实现：

- SQLite 或其他持久化存储；
- daemon 重启后的任务恢复；
- 多队列和并发数量限制；
- 取消任务；
- 多用户、权限和鉴权；
- 优先级、依赖、重试和定时执行；
- 日志追踪、分页或轮转；
- systemd 服务；
- cgroup、namespace、容器和 GPU 调度；
- 网络 worker 和分布式调度。

第一阶段允许多个已提交任务立即并发执行，不承诺公平性或最大并发数。队列调度将在后续阶段单独设计。

## 4. 技术约束

- 使用 Go 标准库完成第一阶段。
- CLI 与 daemon 使用 Unix Domain Socket 通信。
- 消息采用逐行 JSON，即每条 JSON 消息以换行符结束。
- 执行命令使用 `os/exec`，不默认经过 shell。
- 任务存储使用进程内存，并处理并发访问。
- 默认运行目录为 `~/.taskq`，可通过 `TASKQ_HOME` 覆盖，便于测试和隔离。
- Socket 默认位于 `<TASKQ_HOME>/taskqd.sock`。
- 任务日志默认位于 `<TASKQ_HOME>/logs/<job-id>.log`。

## 5. 总体架构

```text
taskq CLI
    │
    │ Unix Domain Socket + newline-delimited JSON
    ▼
taskqd daemon
    │
    ▼
JobService
    ├── JobRepository（第一阶段为内存实现）
    └── Executor（使用 os/exec）
             │
             ▼
         子进程与日志文件
```

系统分为核心层和外部适配层：

- `taskq` 解析命令、发送请求并展示响应，不直接执行任务。
- `taskqd` 监听 Socket、接收请求并组装系统组件。
- `JobService` 负责提交和查询任务，是核心业务入口。
- `JobRepository` 定义核心层需要的存储行为。
- 内存 Repository 是第一阶段的存储实现，后续可替换为 SQLite。
- `Executor` 启动和等待子进程，并把输出写入日志文件。
- `protocol` 定义 CLI 与 daemon 之间稳定的请求和响应格式。

`JobService` 不依赖 Unix Socket 或具体数据库。这样可以独立测试核心逻辑，也方便未来替换外部适配器或使用 Rust 重构。

## 6. 建议目录结构

```text
taskq/
├── cmd/
│   ├── taskq/
│   │   └── main.go
│   └── taskqd/
│       └── main.go
├── internal/
│   ├── job/
│   │   ├── model.go
│   │   ├── repository.go
│   │   └── service.go
│   ├── executor/
│   │   └── process.go
│   ├── storage/
│   │   └── memory.go
│   ├── protocol/
│   │   └── message.go
│   └── daemon/
│       └── server.go
├── docs/
├── go.mod
└── README.md
```

目录和文件按学习进度逐步创建，不预先生成空文件。

职责约束：

- `cmd` 只负责解析启动参数、组装依赖和启动程序。
- `internal/job` 保存任务模型、存储行为定义和核心用例。
- `internal/executor` 只管理进程，不生成任务 ID，也不处理 IPC。
- `internal/storage` 提供存储实现。
- `internal/protocol` 只描述传输消息，不包含业务规则。
- `internal/daemon` 把协议请求转换为 `JobService` 调用。
- 第一阶段不创建宽泛的 `common`、`utils` 或 `pkg` 目录。

## 7. Job 模型

第一阶段的 Job 包含以下信息：

| 字段 | 含义 |
| --- | --- |
| ID | daemon 内单调递增的任务标识，从 1 开始 |
| Command | 要执行的程序名 |
| Args | 独立保存的参数列表 |
| WorkingDir | 提交任务时 CLI 所在的绝对目录 |
| Status | 当前任务状态 |
| ExitCode | 子进程退出码；未获得退出码时为空 |
| LogPath | 合并 stdout 和 stderr 的日志路径 |
| CreatedAt | 创建时间 |
| StartedAt | 子进程成功启动的时间；未启动时为空 |
| FinishedAt | 任务进入最终状态的时间；未结束时为空 |
| Error | 启动或等待进程失败时的错误摘要 |

第一阶段不包含 Queue、PID、用户、优先级和重试信息。

命令和参数必须分开保存。TaskQ 不把所有输入拼成一个 shell 字符串，从而避免额外的转义规则和意外的 shell 展开。需要 shell 功能时，用户可以明确提交 `sh -c <script>`。

CLI 提交任务时记录当前工作目录，daemon 在该目录启动子进程。子进程继承 daemon 的环境变量；第一阶段不把 CLI 的完整环境变量传给 daemon。

## 8. 状态机

```text
Pending ──子进程启动成功──> Running
Pending ──子进程启动失败──> Failed
Running ──退出码为 0──────> Succeeded
Running ──退出码非 0──────> Failed
Running ──等待进程失败────> Failed
```

状态规则：

- `Pending`、`Running`、`Succeeded` 和 `Failed` 是第一阶段的全部状态。
- `Succeeded` 和 `Failed` 是最终状态，不能再流转到其他状态。
- `StartedAt` 只在子进程成功启动后设置。
- `FinishedAt` 只在进入最终状态时设置。
- 正常退出时保存实际退出码，包括退出码 0。
- 子进程未成功启动时，`ExitCode` 保持为空，并在 `Error` 中保存错误摘要。
- 非法状态流转必须返回错误，不能静默修改 Job。

## 9. 请求与执行数据流

### 9.1 提交任务

```text
CLI 解析 program、args 和当前工作目录
→ 连接 daemon
→ 发送 Run 请求
→ daemon 校验请求
→ JobService 创建 Pending Job
→ Repository 保存 Job
→ daemon 立即返回 job ID
→ JobService 在后台调用 Executor
→ Executor 成功启动后更新为 Running
→ stdout 和 stderr 写入同一个日志文件
→ 等待子进程结束
→ 更新为 Succeeded 或 Failed
```

提交是异步操作。`taskq run` 返回任务已经被 daemon 接收，不等待命令完成。

### 9.2 查询任务

```text
taskq ps
→ 发送 List 请求
→ JobService 从 Repository 读取快照
→ daemon 返回任务列表
→ CLI 以表格形式显示
```

列表至少显示 ID、COMMAND、STATUS、CREATED 和 FINISHED。任务按 ID 递增排序，避免内存映射的无序结果影响用户体验和测试稳定性。

### 9.3 查看日志

```text
taskq logs <job-id>
→ 发送 Logs 请求
→ daemon 查找任务及日志路径
→ daemon 读取日志内容并返回
→ CLI 原样写到 stdout
```

第一阶段只返回当时已有的完整日志，不支持持续跟随。stdout 和 stderr 合并到同一个文件，降低第一阶段的协议和展示复杂度。

## 10. IPC 协议

第一阶段使用“一次连接、一次请求、一次响应”的模式：

1. CLI 建立 Unix Socket 连接。
2. CLI 写入一条以换行结束的 JSON 请求。
3. daemon 解码并处理请求。
4. daemon 写入一条以换行结束的 JSON 响应。
5. 双方关闭连接。

请求类型只有：

- `run`：携带 command、args 和 working directory；
- `list`：不携带额外数据；
- `logs`：携带 job ID。

响应统一包含成功标记和可选错误信息，并按请求携带 job、jobs 或 log 数据。协议层错误使用稳定、可判断的错误类型；面向用户的文本由 CLI 组织。

第一阶段不做协议版本协商、流式响应和长连接复用。

## 11. 并发与资源所有权

- daemon 可以同时处理多个短连接。
- 内存 Repository 必须保护任务集合和 ID 分配，避免数据竞争。
- 查询操作返回 Job 值的快照，不能让调用者直接修改 Repository 内部数据。
- 每次提交任务都会启动一个后台执行流程。
- 日志文件由 Executor 创建和关闭。
- 子进程等待由启动该进程的执行流程负责，避免产生僵尸进程。
- daemon 退出时第一阶段不承诺等待或终止仍在运行的子进程；该行为将在“取消与优雅退出”阶段设计。

实现过程中必须运行 Go race detector 验证核心并发路径。

## 12. 错误处理

- CLI 参数不合法：CLI 在连接 daemon 前提示用法并以非零状态退出。
- daemon 无法连接：CLI 明确提示 Socket 路径和连接错误。
- 请求 JSON 无法解码：daemon 返回协议错误，不创建任务。
- command 为空或 working directory 无效：daemon 返回校验错误，不创建任务。
- 命令不存在或权限不足：任务从 `Pending` 进入 `Failed`，保留错误摘要。
- 子进程返回非零退出码：任务进入 `Failed`，保存退出码；这不是 daemon 自身故障。
- 任务不存在：`logs` 返回可判断的 not-found 错误。
- 日志尚未创建：返回日志暂不可用错误，不能把它误报为任务不存在。
- 单个请求或任务失败：记录错误并结束该请求或任务，daemon 继续运行。
- daemon 意外退出：内存任务状态丢失，这是第一阶段已接受的限制。

底层错误需要保留上下文，但 CLI 不输出 Go 调用栈或内部结构。

## 13. 测试策略

### 13.1 单元测试

- `job`：验证合法状态流转、非法状态流转和时间字段。
- `storage`：验证 ID 分配、保存、读取、列表排序和并发访问。
- `executor`：验证成功命令、非零退出、命令不存在和日志写入。
- `service`：使用可替换的 Repository 和 Executor 验证提交及结果更新。
- `protocol`：验证每种请求与响应可以正确进行 JSON 往返转换。

测试不得依赖系统 Python。需要外部命令时只使用 macOS 和 Linux 都具备的基础 Unix 命令，或者使用 Go 测试进程自身作为辅助进程。

### 13.2 集成测试

使用临时目录作为 `TASKQ_HOME`：

1. 启动临时 daemon。
2. 等待 Socket 可连接。
3. 提交一个输出 `hello` 的任务。
4. 查询并等待任务进入最终状态。
5. 验证任务为 `Succeeded`。
6. 验证日志中包含 `hello`。
7. 关闭测试 daemon 并清理临时目录。

测试不能使用固定的用户目录或遗留的 Socket 文件。

### 13.3 每步验证命令

每完成一个小步骤至少执行：

```bash
gofmt -w <本次修改的 Go 文件>
go test ./...
go vet ./...
```

涉及并发后增加：

```bash
go test -race ./...
```

## 14. 学习式开发进度

每个里程碑采用“概念讲解 → 小任务 → 学习者实现 → 测试 → 代码审阅”的循环。进入下一步前，当前步骤必须能够独立解释并通过测试。

### 里程碑 0：整理项目基线

- 统一 README 中当前 Go 实现与未来 Rust 重构的描述。
- 确认 `go.mod` 和本机 Go 工具链。
- 建立基础格式化、测试和静态检查习惯。

完成标准：仓库结构清楚，基础 Go 命令可运行。

### 里程碑 1：Job 模型与状态机

- 学习 package、导出规则、自定义类型、结构体和时间值。
- 定义 Job 和 Status。
- 实现受约束的状态流转。
- 为合法与非法流转编写表驱动测试。

完成标准：状态机测试通过，调用者不能通过业务方法制造非法状态。

### 里程碑 2：Repository 与内存存储

- 学习小接口、指针和值、map、slice、mutex 和错误值。
- 在核心包定义 Repository 所需行为。
- 实现并发安全的内存存储。

完成标准：创建、读取和有序列表测试通过，race detector 不报告数据竞争。

### 里程碑 3：Executor

- 学习 `os/exec`、文件描述符、退出码和资源关闭。
- 执行 program 与独立 args。
- 在指定工作目录运行命令。
- 合并 stdout、stderr 到任务日志。

完成标准：成功、非零退出、启动失败和日志测试通过。

### 里程碑 4：JobService

- 学习依赖注入、goroutine、错误传播和业务编排。
- 提交并保存 Pending Job。
- 后台调用 Executor。
- 根据执行生命周期更新状态。

完成标准：提交立即返回 ID，任务最终状态和日志结果正确。

### 里程碑 5：JSON 协议

- 学习结构体标签、编码解码和协议边界。
- 定义 run、list、logs 请求及统一响应。
- 添加 JSON 往返测试和错误请求测试。

完成标准：协议结构不依赖 CLI 展示文本，所有消息测试通过。

### 里程碑 6：Unix Socket daemon

- 学习 `net.Listener`、`net.Conn`、并发连接和优雅的资源清理。
- 监听 Socket 并清理失效的旧 Socket 文件。
- 将请求转发给 JobService。

完成标准：测试客户端可以完成 run、list、logs 请求，错误请求不会使 daemon 退出。

### 里程碑 7：CLI

- 学习 `os.Args`、退出码、stdout/stderr 分工和表格输出。
- 实现 `taskq run`、`taskq ps` 和 `taskq logs`。
- 对参数错误和 daemon 不可用给出明确提示。

完成标准：用户可以只通过 CLI 完成第一阶段最小闭环。

### 里程碑 8：端到端验收

- 在临时目录运行完整集成测试。
- 在 macOS 手动完成最小闭环。
- 在 Linux 上验证构建和运行行为。
- 更新 README 的使用方法与已知限制。

完成标准：本文档第 15 节的验收场景全部通过。

## 15. 第一阶段验收场景

### 15.1 成功任务

```text
启动 taskqd
运行 taskq run echo hello
获得 job ID
运行 taskq ps
观察任务最终为 Succeeded
运行 taskq logs <job-id>
观察输出包含 hello
```

### 15.2 非零退出任务

提交一个返回非零退出码的命令后：

- daemon 继续运行；
- Job 最终状态为 `Failed`；
- Job 保存实际退出码；
- 后续 `ps` 和 `logs` 请求仍然可用。

### 15.3 启动失败任务

提交一个不存在的程序后：

- `run` 返回已创建的 job ID；
- Job 从 `Pending` 进入 `Failed`；
- `ExitCode` 为空；
- `Error` 中包含可理解的启动失败摘要；
- daemon 继续运行。

### 15.4 daemon 不可用

daemon 未启动时运行 CLI：

- CLI 返回非零退出状态；
- 错误信息包含尝试连接的 Socket 路径；
- CLI 不创建本地 Job 或日志。

## 16. 第一行代码从哪里开始

设计文档经用户审阅后，从 `internal/job/model.go` 开始，而不是从 CLI 或 daemon 入口开始。

第一项编码任务是定义 Job 状态及其流转规则，并先通过测试固定这些行为。后续所有组件都围绕已验证的 Job 模型逐层组装。

详细到文件、测试和验证命令的实施计划将在本文档获批后单独编写。
