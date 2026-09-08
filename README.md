# TaskQ 系统设计文档

## 1. 项目概述

TaskQ 是一个运行于 Linux 主机上的轻量级任务调度系统，用于管理本地计算任务。

目标是提供类似 Slurm、Jenkins Worker、Celery Worker 的核心能力，但保持：

* 单机部署
* 低资源占用
* 简单可靠
* 易扩展

用户通过 CLI 提交任务，由后台 daemon 负责：

* 接收任务
* 持久化任务状态
* 调度执行
* 管理并发
* 收集日志
* 控制任务生命周期

---

# 2. 设计目标

## 2.1 功能目标

支持：

* 多队列管理
* 任务提交
* 任务状态查询
* 任务取消
* 日志查看
* 并发限制
* 任务持久化
* daemon 开机启动

示例：

```bash
taskq run gpu python train.py

taskq ps

taskq logs 1001

taskq stop 1001
```

---

## 2.2 非目标

第一阶段不考虑：

* 多机器调度
* 分布式一致性
* 容器编排
* 网络 worker 集群

这些能力可以在后续版本扩展。

---

# 3. 总体架构

```
                +-------------+
                |  taskq CLI  |
                +-------------+
                       |
                       |
              Unix Domain Socket
                       |
                       v

                +-------------+
                |   taskqd    |
                |   daemon    |
                +-------------+

          +-----------+-----------+
          |           |           |
          v           v           v

      Scheduler    Storage    Executor

          |           |           |
          |           |           |
          v           v           v

      Queue      SQLite      Child Process


```

---

# 4. 技术选型

## 4.1 编程语言

Rust

原因：

* 内存安全
* 并发安全
* 系统级能力
* 适合长期运行 daemon

---

## 4.2 核心依赖

| 功能     | 技术                 |
| ------ | ------------------ |
| 异步运行时  | Tokio              |
| CLI    | Clap               |
| 数据库    | SQLite             |
| ORM/DB | SQLx 或 rusqlite    |
| 序列化    | Serde              |
| 日志     | tracing            |
| IPC    | Unix Domain Socket |

---

# 5. 项目结构

```
taskq/

├── crates/

│
├── taskq-cli/
│   └── main.rs
│
├── taskqd/
│   └── main.rs
│
├── scheduler/
│   └── scheduler.rs
│
├── executor/
│   └── executor.rs
│
├── storage/
│   └── sqlite.rs
│
├── protocol/
│   └── message.rs
│
├── common/
│   └── types.rs
│
├── config/
│
└── systemd/

```

```
taskq/

├── cmd/
│
│   ├── taskq/
│   │   └── main.go
│   │
│   └── taskqd/
│       └── main.go
│
├── internal/
│
│   ├── daemon/
│   │   └── server.go
│   │
│   ├── scheduler/
│   │   └── scheduler.go
│   │
│   ├── queue/
│   │   └── queue.go
│   │
│   ├── job/
│   │   └── job.go
│   │
│   ├── executor/
│   │   └── executor.go
│   │
│   ├── storage/
│   │   └── sqlite.go
│   │
│   └── protocol/
│       └── message.go
│
├── go.mod
└── README.md
```

---

# 6. 核心数据模型

## 6.1 Job

```rust
struct Job {

    id: i64,

    queue: String,

    command: String,

    args: Vec<String>,

    status: JobStatus,

    pid: Option<u32>,

    created_at: DateTime,

    started_at: Option<DateTime>,

    finished_at: Option<DateTime>,

}
```

状态：

```rust
enum JobStatus {

    Pending,

    Running,

    Completed,

    Failed,

    Cancelled,

}
```

---

## 6.2 Queue

```rust
struct Queue {

    name: String,

    max_workers: usize,

    pending_jobs: Vec<JobId>,

    running_jobs: Vec<JobId>,

}
```

示例：

```
gpu:

max_workers = 1

running:
    1001

pending:
    1002
    1003


build:

max_workers = 4

running:
    2001
    2002

```

---

# 7. 数据存储设计

采用 SQLite 作为持久化层。

## jobs 表

```sql
CREATE TABLE jobs (

    id INTEGER PRIMARY KEY,

    queue TEXT NOT NULL,

    command TEXT NOT NULL,

    status TEXT NOT NULL,

    pid INTEGER,

    created_at INTEGER,

    started_at INTEGER,

    finished_at INTEGER

);
```

---

## queues 表

```sql
CREATE TABLE queues (

    name TEXT PRIMARY KEY,

    max_workers INTEGER

);
```

---

## logs 表

```sql
CREATE TABLE logs (

    job_id INTEGER,

    stdout_path TEXT,

    stderr_path TEXT

);
```

---

# 8. CLI 设计

## 提交任务

命令：

```bash
taskq run gpu python train.py
```

流程：

```
CLI

 |
 |
Unix Socket

 |
 |
taskqd

 |
 |
create Job

 |
 |
insert SQLite

 |
 |
scheduler dispatch

```

---

## 查询任务

```bash
taskq ps
```

输出：

```
ID      QUEUE     STATUS

1001    gpu       RUNNING

1002    gpu       PENDING

```

---

## 停止任务

```bash
taskq stop 1001
```

daemon：

```
lookup job

      |

find pid

      |

SIGTERM

      |

update status

```

---

# 9. 调度器设计

Scheduler 负责：

* 检查队列
* 分配 worker slot
* 启动任务

核心逻辑：

```
loop:

    load queues

    for queue:

        while available_slot:

            job = next_pending()

            executor.start(job)


    sleep(interval)

```

---

# 10. Executor 设计

Executor 负责：

* 创建进程
* 管理 stdout/stderr
* 监听退出

Rust:

```rust
Command::new(program)
    .args(args)
    .spawn();
```

任务结构：

```
taskqd

   |

   +--- job 1001

   |

   +--- job 1002

```

---

# 11. 进程生命周期

## 创建

```
Pending

   |

scheduler dispatch

   |

Running

```

## 完成

```
Running

   |

child exit

   |

SIGCHLD/event

   |

Completed

```

---

# 12. IPC 协议设计

通信方式：

```
Unix Domain Socket

/run/taskq/taskqd.sock

```

消息：

```rust
enum Request {

    Submit(JobRequest),

    List,

    Status(JobId),

    Cancel(JobId),

}
```

响应：

```rust
enum Response {

    Success,

    Error(String),

    Jobs(Vec<Job>),

}
```

---

# 13. 配置文件

路径：

```
/etc/taskq/config.toml

```

示例：

```toml
[data]
path="/var/lib/taskq"


[queue.gpu]
workers=1


[queue.build]
workers=4

```

---

# 14. 日志设计

目录：

```
/var/log/taskq/

1001.stdout

1001.stderr

```

日志系统：

* tracing
* log rotation

---

# 15. 权限设计

限制：

* 用户只能管理自己的任务
* queue 权限控制
* 工作目录隔离

未来支持：

* namespace
* cgroup
* container executor

---

# 16. Systemd 集成

服务：

```
/etc/systemd/system/taskqd.service

```

启动：

```bash
systemctl enable taskqd

systemctl start taskqd
```

---

# 17. 开发路线

## Phase 1：核心版本

实现：

* daemon
* CLI
* Unix socket
* fork/exec
* 状态查询

目标：

```bash
taskq run test echo hello
```

---

## Phase 2：调度系统

增加：

* 多 queue
* worker limit
* SQLite
* cancel

---

## Phase 3：生产化

增加：

* 日志系统
* 权限控制
* systemd
* 配置管理

---

## Phase 4：高级能力

增加：

* priority
* retry
* dependency
* GPU binding
* container executor

---

# 18. 长期演进方向

最终目标：

```
TaskQ

 |
 +-- Local Scheduler

 |
 +-- GPU Scheduler

 |
 +-- Container Runtime

 |
 +-- Web Dashboard

 |
 +-- Distributed Worker

```

TaskQ 可以逐步演进为一个轻量级计算任务平台。
