# dream

`dream` 是一个用 Go 实现的命令行工具，用来驱动 `codex` 或 `claude` 按轮次执行同一份工作流文档。

它的核心目标不是承载业务逻辑，而是把下面几件事稳定地串起来：

- 读取一份文档化的工作流说明
- 检查并补全 `PROJECT_ROOT`
- 每轮只执行一个当前最应该做的任务
- 每轮完成后更新同一份文档
- 每轮之间休眠 20 秒
- 下一轮强制使用新的 session，避免上下文膨胀
- 在 `.dream/` 下保留完整运行状态和日志

## 本地编译

当前仓库使用 Go 1.26，见 `go.mod`。

在仓库根目录执行：

```bash
go build -buildvcs=false -o ../bin/dream .
```

说明：

- 产物会生成在上一级目录的 `bin/` 下，文件名为 `dream`
- 这里显式加了 `-buildvcs=false`，避免本地仓库状态导致 `go build` 的 VCS stamping 报错
- 构建完成后可直接用 `../bin/dream -h` 或 `../bin/dream run -h` 验证帮助输出

## 1. CLI 如何使用

### 1.1 命令总览

当前版本支持 5 个主命令：

```bash
dream run [--model codex|claude] [--settings path-or-json] <doc>
dream resume <doc-or-workflow>
dream status [doc-or-workflow]
dream logs <doc-or-workflow> [--round N]
dream template <dir>
```

说明：

- `doc` 是工作流文档路径
- `doc-or-workflow` 可以是文档路径，也可以是 workflow id
- `--model/-m` 用于指定执行后端；不指定时默认 `codex`
- `--settings/-s` 仅在 `--model claude` 时生效，对应原生 `claude --settings`
- `--round N` 用于查看指定轮次日志；不指定时默认查看当前轮次
- `dir` 是要创建的模板目录；相对路径默认创建在当前执行命令的工作目录下

### 1.2 最常见的使用模式

#### 模式一：从零开始执行一个工作流

```bash
dream run ./task.md
```

使用 Claude：

```bash
dream run -m claude ./task.md
dream run -m claude -s ~/.claude/settings-ice.json ./task.md
```

适用场景：

- 你已经准备好一份任务文档
- 文档里描述了项目背景、当前任务和进度要求
- 希望 `dream` 自动分轮调用 `codex` 或 `claude`

执行时会发生什么：

1. `dream` 读取文档
2. 在文档靠前位置检查 `PROJECT_ROOT=...`
3. 如果缺失或无效，自动使用当前执行命令时的工作目录作为项目根目录，并写回文档头部
4. 调用对应后端的非交互模式
5. 本轮结束后判断是否还有后续任务
6. 如果还有，则等待 20 秒，再开启新 session 进入下一轮
7. 直到文档任务全部完成

#### 模式二：恢复一个中断的工作流

```bash
dream resume ./task.md
```

也可以：

```bash
dream resume f2bb6297e67215d2
```

适用场景：

- 上一轮执行时被手动中断
- 进程异常退出
- 想继续当前 workflow，而不是重新开始

恢复行为：

- 优先恢复当前未完成轮次对应的 session
- 当前轮次完成后，`dream` 仍然会继续后续轮次
- 后续轮次仍然是新 session，不复用前一轮上下文

#### 模式三：查看所有工作流状态

```bash
dream status
```

输出包含：

- workflow id
- model
- 当前状态
- 当前轮次
- 当前 session id
- 对应文档路径

适用场景：

- 想快速知道本目录下有哪些 `dream` 工作流
- 想确认某个 workflow 当前是否完成、中断或失败

#### 模式四：查看某个工作流的详细状态

```bash
dream status ./task.md
```

或：

```bash
dream status f2bb6297e67215d2
```

输出包含：

- workflow id
- model
- state
- current round
- current session id
- doc path
- project root
- created_at
- updated_at

#### 模式五：查看当前轮次或历史轮次日志

```bash
dream logs ./task.md
```

查看指定轮次：

```bash
dream logs ./task.md --round 2
```

适用场景：

- 想回看某一轮的最终助手输出
- 想定位是哪一轮出现异常

#### 模式六：快速生成一套计划文档模板

```bash
dream template my-plan
```

生成后可直接开始执行：

```bash
dream run my-plan/README.md
```

适用场景：

- 想快速搭一个可长期维护的工作流目录
- 希望总文档和子文档结构从一开始就对齐
- 不想每次手工重新抄计划骨架

生成内容：

- `my-plan/README.md`
- `my-plan/feature-foundation.md`

说明：

- 模板生成的是可修改的初始骨架，不是固定死的严格格式
- 首次执行 `dream run my-plan/README.md` 时，`dream` 会自动补写 `PROJECT_ROOT=...`
- 目标目录必须不存在或为空目录；如果目录里已有文件，`dream template` 会直接拒绝覆盖

## 2. 文档格式与使用约定

`dream` 不要求你把文档写成严格的固定模板，但至少需要满足两点：

1. 文档中应明确描述当前工作内容与进度
2. 文档靠前位置最终应存在 `PROJECT_ROOT=/absolute/path`

如果文档里没有合法的 `PROJECT_ROOT`，`dream` 会在首次 `run` 时自动把当前工作目录写回文档头部。因此最稳妥的使用方式，是在项目根目录执行 `dream run`。

一个最小可用示例：

```md
PROJECT_ROOT=/absolute/path/to/project

# 工作流文档

目标：
- 修复解析逻辑
- 补充测试

当前进度：
- 未完成：修复 parser 中的边界条件
- 未完成：补充对应单元测试
```

在执行过程中，执行后端会继续修改这同一份文档，把它同时作为：

- 提示词文档
- 方案文档
- 进度文档

如果你不想手写初始骨架，也可以先执行：

```bash
dream template my-plan
dream run my-plan/README.md
```

生成模板后，再按项目实际情况补充或重命名功能点文档即可。

## 3. 这个 CLI 会产出什么

`dream` 的所有运行时产物默认都放在当前目录的 `.dream/` 下。

### 3.1 总体目录结构

```text
.dream/
  index.json
  workflows/
    <workflow_id>/
      meta.json
      status.json
      lock
      rounds/
        0001/
          prompt.txt
          session_id.txt
          last.txt
          stdout.log
          stderr.log
        0002/
          ...
```

### 3.2 产物说明

#### `index.json`

作用：

- 保存当前目录下所有 workflow 的索引
- 让 `dream status` 不需要全量扫描所有目录

如何使用：

- 快速查看有哪些 workflow
- 做自定义脚本集成时读取这个索引即可

#### `workflows/<workflow_id>/meta.json`

作用：

- 保存 workflow 的基础元数据
- 记录文档路径和项目根目录

如何使用：

- 判断某个 workflow 绑定的是哪份文档
- 还原 workflow 与项目目录的映射关系

#### `workflows/<workflow_id>/status.json`

作用：

- 保存 workflow 当前状态
- 记录当前轮次、当前 session id 和时间戳

如何使用：

- 判断当前是 `created`、`running`、`interrupted`、`completed` 还是 `failed`
- 恢复或排查故障时优先看这个文件

#### `workflows/<workflow_id>/lock`

作用：

- 防止同一个 workflow 被并发执行两次

如何使用：

- 通常不需要手动操作
- 如果异常退出后残留，可以结合 `status.json` 判断是否需要人工清理

#### `rounds/<NNNN>/prompt.txt`

作用：

- 保存该轮真正发给 Codex 的控制 prompt

如何使用：

- 排查为什么这一轮会执行某个任务
- 回溯每轮的驱动规则是否发生变化

#### `rounds/<NNNN>/session_id.txt`

作用：

- 保存该轮绑定的 Codex session id

如何使用：

- 对照 `resume` 行为
- 排查某一轮是否确实使用了新的 session

#### `rounds/<NNNN>/last.txt`

作用：

- 保存该轮最后一条助手消息
- `dream` 就是通过它来解析 `TASK_DONE` / `NEXT_TASK` / `ALL_TASKS_DONE`

如何使用：

- 快速看该轮最终结论
- 检查协议标记是否正确输出

#### `rounds/<NNNN>/stdout.log`

作用：

- 保存 `codex` 的标准输出

当前实现中的实际内容：

- 通常主要是最后一条助手消息
- 常见情况下不包含完整的终端过程输出

如何使用：

- 快速查看该轮的最终总结
- 快速查看 `TASK_DONE`、`NEXT_TASK`、`ALL_TASKS_DONE`

#### `rounds/<NNNN>/stderr.log`

作用：

- 保存 `codex` 的标准错误输出

当前实现中的实际内容：

- 启动 banner
- `session id`
- `user`/`codex` 对话过程
- `exec` 工具调用过程
- `apply patch` 等过程性输出

如何使用：

- 当你想看“终端里到底打印了什么”时，优先看这个文件
- 当你要排查 session、工具调用、执行过程时，优先看这个文件

### 3.3 这些产物怎么配合使用

一个常见排查顺序如下：

1. 先看 `dream status`
2. 再看对应 workflow 的 `status.json`
3. 如果要看最终结论，看 `last.txt` 或 `stdout.log`
4. 如果要看完整过程，看 `stderr.log`
5. 如果要追踪会话切换，看每轮的 `session_id.txt`
6. 如果要确认这一轮实际发给 Codex 的规则，看 `prompt.txt`

### 3.4 如果模型误判任务已完成，如何人工重置

有时模型可能错误输出了 `ALL_TASKS_DONE`，导致 `dream` 把整个 workflow 标记成 `completed`。

如果你确认任务实际上还没有完成，推荐的处理方式不是 `dream resume`，而是：

1. 清除这次误判产生的完成状态
2. 准备好文档和运行状态
3. 用 `dream run <doc>` 重新开始下一轮执行

原因是：

- `resume` 的语义是继续当前中断轮次
- 误判 `ALL_TASKS_DONE` 说明上一轮已经被当作“正常结束”
- 这时重新用 `dream run` 启动新一轮，更符合 `dream`“每轮新 session”的设计

推荐按下面步骤操作。

#### 第一步：定位目标 workflow

先查看工作流状态：

```bash
dream status ./task.md
```

然后找到对应的：

- workflow id
- 文档路径
- `.dream/workflows/<workflow_id>/status.json`

#### 第二步：清除任务的完成状态

打开：

```text
.dream/workflows/<workflow_id>/status.json
```

把其中的：

```json
"state": "completed"
```

改成：

```json
"state": "failed"
```

说明：

- 这里推荐改成 `failed`，表示这次“完成”结果不可信，需要人工重置后重新跑
- 不要改成 `interrupted`，否则下一次更适合用的是 `dream resume`
- 一般不要清空 `current_round`
- 一般不要清空 `current_session_id`

如果你希望 `dream status` 在不带参数时立刻显示新的状态，也可以顺手把 `.dream/index.json` 里对应 workflow 的 `state` 同步改成 `failed`。

如果不改 `index.json` 也可以，下一次 `dream` 正常保存状态时会自动刷新索引。

#### 第三步：准备好再次使用

在重新启动前，建议同时检查这几项：

1. 修改工作流文档，把未完成事项、人工判断结果和下一步任务写清楚
2. 确认文档里的 `PROJECT_ROOT` 仍然有效
3. 如果 `.dream/workflows/<workflow_id>/lock` 还在，而且确认没有其他 `dream` 进程正在运行，就把它删掉

例如：

```bash
rm .dream/workflows/<workflow_id>/lock
```

只有在确认当前没有别的 `dream` 进程还在执行这个 workflow 时，才应该删除这个锁文件。

#### 第四步：使用 `dream run` 重新开始执行

准备完成后，重新启动：

```bash
dream run ./task.md
```

这次运行会：

- 保留已有的 workflow 目录和历史轮次日志
- 从当前 `status.json` 的基础上继续进入下一轮
- 为新一轮创建新的 session，而不是复用之前那一轮的会话

这也是处理“误判已完成”最稳妥的方式：保留历史痕迹，但把 workflow 从“错误完成”状态重新拉回可执行状态。

## 4. 这个 CLI 是怎么实现的

### 4.1 实现原理

`dream` 的实现可以理解为 4 个层次：

#### 1. CLI 层

负责：

- 解析命令行参数
- 分发 `run` / `resume` / `status` / `logs`
- 输出人类可读结果

#### 2. 文档层

负责：

- 读取工作流文档
- 检查文档前部是否存在合法 `PROJECT_ROOT`
- 如果没有，则自动使用当前工作目录
- 将 `PROJECT_ROOT` 写回文档头部

#### 3. 状态层

负责：

- 在 `.dream/` 下创建 workflow 目录和轮次目录
- 保存 `meta.json`、`status.json`、`index.json`
- 管理锁文件
- 为每一轮生成独立的日志和 session 文件

#### 4. 执行层

负责：

- 组装每轮 prompt
- 调用 `codex` 或 `claude` 的非交互接口
- 恢复时调用对应后端的 resume 能力
- 收集 stdout/stderr
- 记录或恢复 session id
- 从 `last.txt` 里解析轮次结果

### 4.2 每一轮是怎么推进的

`dream` 不自己理解业务任务内容，而是通过“文档 + 协议标记”推进流程。

每一轮大致如下：

1. 读取当前文档和 workflow 状态
2. 构造一段控制 prompt，明确要求：
   - 只做当前最应该做的一个任务
   - 完成后更新同一份文档
   - 输出 `TASK_DONE` / `NEXT_TASK` / `ALL_TASKS_DONE`
3. 调用当前模型后端
4. 记录日志和 session id
5. 读取 `last.txt`
6. 解析本轮是否完成、是否还有下一轮
7. 如果还有下一轮，等待 20 秒
8. 开启一个新的 session 进入下一轮

其中最重要的设计点是：

- 文档是业务语义的真相源
- `.dream/` 是运行状态的真相源
- 每轮都是新 session，避免上下文越来越大
- `resume` 只恢复当前中断轮次，之后再继续正常轮转

### 4.3 架构 ASCII 图

#### 总体架构图

```text
                +----------------------+
                |      用户终端         |
                | dream run/resume/... |
                +----------+-----------+
                           |
                           v
                +----------------------+
                |      dream CLI       |
                | 参数解析 / 命令分发   |
                +----------+-----------+
                           |
          +----------------+----------------+
          |                                 |
          v                                 v
+----------------------+        +----------------------+
|      文档处理层       |        |      状态存储层       |
| 读取文档 / 校验路径    |        | .dream/index/status  |
| 补写 PROJECT_ROOT     |        | rounds/log/session   |
+----------+-----------+        +----------+-----------+
           |                                 |
           +----------------+----------------+
                            |
                            v
                 +------------------------+
                 |      执行器封装         |
                 | codex / claude         |
                 | stdout/stderr/last.txt |
                 +-----------+------------+
                             |
                             v
                 +------------------------+
                 |      模型 CLI 层        |
                 |   执行一轮实际任务      |
                 +-----------+------------+
                             |
                             v
                 +------------------------+
                 |   项目代码 + 工作流文档  |
                 |   修改代码 / 更新文档    |
                 +------------------------+
```

#### 轮次推进图

```text
     +--------------------+
     | 读取工作流文档和状态 |
     +----------+---------+
                |
                v
     +--------------------+
     | 生成本轮控制 prompt |
     +----------+---------+
                |
                v
     +--------------------+
     | 调用当前后端 run   |
     | 或当前后端 resume  |
     +----------+---------+
                |
                v
     +--------------------+
     | 写入 round 产物     |
     | prompt/log/last/id |
     +----------+---------+
                |
                v
     +--------------------+
     | 解析 last.txt      |
     +----------+---------+
                |
       +--------+--------+
       |                 |
       v                 v
 +-------------+   +------------------+
 | ALL_TASKS   |   | TASK_DONE /      |
 | _DONE       |   | NEXT_TASK        |
 +------+------+   +--------+---------+
        |                    |
        v                    v
 +-------------+      +----------------+
 | 标记完成     |      | 睡眠 20 秒      |
 | state=done  |      +--------+-------+
 +-------------+               |
                               v
                     +--------------------+
                     | 新建 session 进下轮 |
                     +--------------------+
```

## 5. 当前实现的几个重要说明

### 5.1 关于 `stdout.log` 和 `stderr.log`

当前实现保持了底层真实分流：

- `stdout.log` 保存 `codex` 的 stdout
- `stderr.log` 保存 `codex` 的 stderr

因此如果你想看“最终总结”，通常看 `stdout.log`。

如果你想看“完整执行过程”，通常看 `stderr.log`。

### 5.2 关于完成判定

`dream` 不是靠理解业务内容来判断一轮是否结束，而是靠协议标记：

- `TASK_DONE: ...`
- `NEXT_TASK: ...`
- `ALL_TASKS_DONE`

如果 Codex 没有输出这些标记，这一轮就会被视为协议异常。

### 5.3 关于 session 切换

每一轮必须使用新的 session。

这样做的目的：

- 限制上下文增长
- 降低长会话污染后续判断的概率
- 让每一轮都只基于“当前文档 + 当前代码”重新判断下一步

## 6. 建议的实际使用方式

如果你把 `dream` 用在真实项目里，推荐这样操作：

1. 准备一份工作流文档，或先用 `dream template <dir>` 生成模板
2. 先用 `dream run <doc>` 启动
3. 用 `dream status` 跟踪整体进度
4. 用 `dream logs <doc>` 看每轮结果
5. 需要看完整执行过程时，直接打开 `.dream/.../stderr.log`
6. 被中断时，用 `dream resume <doc>` 继续

这样你会同时得到三类信息：

- 文档里的业务进度
- `.dream/` 里的运行状态
- `stdout.log` / `stderr.log` 里的轮次日志
