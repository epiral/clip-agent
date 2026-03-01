# clip-agent

Epiral Agent 的 Clip 化实现。纯本地，SQLite 驱动，流式输出，无 Zulip 依赖。

## 核心概念

### Topic

对话的命名空间。一个 Topic 对应一个持续的对话上下文（如"代码审查"、"旅行规划"）。  
所有 Run 都归属于某个 Topic。

### Run = Agentic Loop

**Run 是核心模型。**

一个 Run 代表一次完整的 agentic loop 周期：

```
用户 message 进来（触发）
    ↓
LLM 响应 → 可能包含 tool_calls
    ↓
执行工具 → tool_result 追加进 messages
    ↓
LLM 再次响应 → 可能再次调用工具
    ↓
... 循环迭代 ...
    ↓
LLM stop_reason = end_turn → Run 结束
```

`run.messages` 存储该 agentic loop 的**完整执行轨迹（Trajectory）**：

```json
[
  {"role": "user",      "content": "帮我分析这段代码"},
  {"role": "assistant", "content": null, "tool_calls": [{"name": "read_file", "arguments": "..."}]},
  {"role": "tool",      "content": "文件内容..."},
  {"role": "assistant", "content": null, "tool_calls": [{"name": "search", "arguments": "..."}]},
  {"role": "tool",      "content": "搜索结果..."},
  {"role": "assistant", "content": "根据以上分析，问题在于..."}
]
```

### Run 链表

同一 Topic 下的 Run 通过 `prev_run_id` 形成单链表，追溯完整对话历史：

```
topic
  └── run_1  (prev=null)    ← 第一轮
  └── run_2  (prev=run_1)
  └── run_3  (prev=run_2)   ← 当前最新
```

### 上下文构建

`send-message` 触发新 Run 时，上下文构建逻辑：

```
1. 读取前序 Run 的完整 messages 轨迹（通过 prev_run_id 追溯）
2. append 本次 user message
3. 开始新的 agentic loop（LLM 调用 → 工具执行 → 迭代）
4. loop 结束后，一次性将完整 messages 存入新 Run（不在流式过程中写库）
```

MVP 阶段无工具调用，每个 Run 的 messages 退化为：
```json
[{"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}]
```
但数据结构天然支持 tool use 扩展，不需要 schema 变更。

## 命令集

通过 `bin/agent --commands` 自省发现所有可用命令。

| 命令 | stdin | stdout | 类型 |
|------|-------|--------|------|
| `create-topic` | `{"name": "..."}` | `{id, name, created_at}` | 非流式 |
| `list-topics` | 空 | `[{id, name, run_count, last_run_at}]` | 非流式 |
| `get-runs` | `{"topic_id": "..."}` | `[{id, user_message, messages, created_at}]` | 非流式 |
| `send-message` | `{"topic_id": "...", "message": "..."}` | token 流 | **流式** |

## 技术约束

- **流式输出**：`send-message` 通过 `os.Stdout` 逐 token flush，Pinix `InvokeStream` RPC 负责推送给 Web UI
- **DB 写入**：Run 的 messages 在 agentic loop 结束后一次性写入，流式期间不写库
- **上下文窗口**：通过 `prev_run_id` 链追溯历史，控制传入 LLM 的 messages 数量上限

## 目录结构

```
clip-agent/
  bin/agent          # Go 多子命令二进制
  cmd/agent/main.go  # 主逻辑
  seed/schema.sql    # SQLite schema（18 张表，WAL 模式）
  web/               # Chatbot Web UI（Vite + Tailwind v4）
  data/              # 运行时数据（agent.db，gitignore）
  lib/               # 依赖（vec0.dylib 等）
  clip.yaml          # Clip 声明
```
