# clip-agent

Epiral Agent 的 Clip 化实现。

## 架构

- **Topic** 为第一公民，替代 Zulip stream+topic 坐标系
- **Run** = 一次 user message 触发的完整 agentic loop
- 单用户，零外部依赖（SQLite + sqlite-vec，无 PostgreSQL / Redis / Zulip）

## 目录结构

```
commands/   命令脚本（handle-message, event-check, ...）
web/        Chatbot UI（Vite + Tailwind v4）
data/       运行时数据（SQLite DB，不入 Git）
seed/       初始化数据
  schema.sql  完整 Schema v2.0（18 张表）
lib/        依赖（不入 Git）
```

## Schema

见 `seed/schema.sql`，18 张表：

topics, topic_tags, runs, run_tools, run_vectors, vec_run_vectors,
user_facts, fact_changes, skills, skill_resources, skill_vectors,
vec_skill_vectors, skill_usage, scheduled_events, topic_settings,
topic_summaries, period_summaries, llm_usage
