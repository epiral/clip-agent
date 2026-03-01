-- ============================================================
-- Epiral Agent · Clip Schema v2.0
-- 原则：Topic 第一公民 · 单用户 · 零外部依赖
-- 启用 WAL + 外键约束
-- ============================================================

PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

-- 1. TOPICS
CREATE TABLE IF NOT EXISTS topics (
  id          TEXT    PRIMARY KEY,
  name        TEXT    NOT NULL,
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL,
  archived_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_topics_archived ON topics(archived_at);
CREATE INDEX IF NOT EXISTS idx_topics_created  ON topics(created_at DESC);

-- 2. TOPIC TAGS
CREATE TABLE IF NOT EXISTS topic_tags (
  topic_id  TEXT    NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
  tag       TEXT    NOT NULL,
  tagged_at INTEGER NOT NULL,
  PRIMARY KEY (topic_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_topic_tags_tag ON topic_tags(tag);

-- 3. RUNS
CREATE TABLE IF NOT EXISTS runs (
  id           TEXT    PRIMARY KEY,
  topic_id     TEXT    NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
  prev_run_id  TEXT    REFERENCES runs(id),
  user_message TEXT,
  summary      TEXT,
  messages     TEXT,
  status       TEXT    NOT NULL DEFAULT 'in_progress',
  started_at   INTEGER,
  ended_at     INTEGER,
  created_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_runs_topic  ON runs(topic_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_runs_prev   ON runs(prev_run_id);
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_runs_topic_head
  ON runs(topic_id) WHERE prev_run_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_runs_topic_chain
  ON runs(topic_id, prev_run_id) WHERE prev_run_id IS NOT NULL;

-- 4. RUN TOOLS
CREATE TABLE IF NOT EXISTS run_tools (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id    TEXT    NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  tool_name TEXT    NOT NULL,
  args      TEXT,
  result    TEXT,
  seq       INTEGER NOT NULL,
  called_at INTEGER NOT NULL,
  UNIQUE(run_id, seq)
);
CREATE INDEX IF NOT EXISTS idx_run_tools_run  ON run_tools(run_id);
CREATE INDEX IF NOT EXISTS idx_run_tools_name ON run_tools(tool_name);

-- 5. RUN VECTORS
CREATE TABLE IF NOT EXISTS run_vectors (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id      TEXT    NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  topic_id    TEXT    NOT NULL,
  hash        TEXT    NOT NULL,
  chunk_seq   INTEGER NOT NULL,
  chunk_text  TEXT    NOT NULL,
  embedding   BLOB,
  embedded_at INTEGER,
  UNIQUE(run_id, chunk_seq)
);
CREATE INDEX IF NOT EXISTS idx_run_vectors_run   ON run_vectors(run_id);
CREATE INDEX IF NOT EXISTS idx_run_vectors_topic ON run_vectors(topic_id);
CREATE INDEX IF NOT EXISTS idx_run_vectors_hash  ON run_vectors(hash);

-- vec0 虚拟表，rowid 对应 run_vectors.id
CREATE VIRTUAL TABLE IF NOT EXISTS vec_run_vectors USING vec0(
  embedding float[1536]
);

-- 6. USER FACTS
CREATE TABLE IF NOT EXISTS user_facts (
  id            TEXT PRIMARY KEY,
  category      TEXT NOT NULL,
  content       TEXT NOT NULL,
  confidence    REAL NOT NULL DEFAULT 1.0,
  source_run_id TEXT REFERENCES runs(id) ON DELETE SET NULL,
  expires_at    INTEGER,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_user_facts_category ON user_facts(category);

-- 7. FACT CHANGES
CREATE TABLE IF NOT EXISTS fact_changes (
  id             TEXT    PRIMARY KEY,
  fact_id        TEXT    NOT NULL,
  source_run_id  TEXT    REFERENCES runs(id) ON DELETE SET NULL,
  action         TEXT    NOT NULL,
  before_content TEXT,
  after_content  TEXT,
  reason         TEXT    NOT NULL,
  created_at     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_fact_changes_fact    ON fact_changes(fact_id);
CREATE INDEX IF NOT EXISTS idx_fact_changes_created ON fact_changes(created_at DESC);

-- 8. SKILLS
CREATE TABLE IF NOT EXISTS skills (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT    NOT NULL UNIQUE,
  description TEXT    NOT NULL,
  content     TEXT    NOT NULL,
  scope       TEXT    NOT NULL DEFAULT 'private',
  status      TEXT    NOT NULL DEFAULT 'active',
  version     INTEGER NOT NULL DEFAULT 1,
  metadata    TEXT    NOT NULL DEFAULT '{}',
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_skills_scope_status ON skills(scope, status);

-- 9. SKILL RESOURCES
CREATE TABLE IF NOT EXISTS skill_resources (
  id       INTEGER PRIMARY KEY AUTOINCREMENT,
  skill_id INTEGER NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
  type     TEXT    NOT NULL,
  name     TEXT    NOT NULL,
  content  TEXT,
  metadata TEXT    NOT NULL DEFAULT '{}',
  UNIQUE(skill_id, type, name)
);

-- 10. SKILL VECTORS
CREATE TABLE IF NOT EXISTS skill_vectors (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  skill_id    INTEGER NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
  resource_id INTEGER REFERENCES skill_resources(id) ON DELETE CASCADE,
  source      TEXT    NOT NULL,
  chunk_seq   INTEGER NOT NULL,
  chunk_text  TEXT    NOT NULL,
  embedding   BLOB,
  UNIQUE(skill_id, source, chunk_seq)
);
CREATE INDEX IF NOT EXISTS idx_skill_vectors_skill ON skill_vectors(skill_id);

-- vec0 虚拟表，rowid 对应 skill_vectors.id
CREATE VIRTUAL TABLE IF NOT EXISTS vec_skill_vectors USING vec0(
  embedding float[1536]
);

-- 11. SKILL USAGE
CREATE TABLE IF NOT EXISTS skill_usage (
  id       INTEGER PRIMARY KEY AUTOINCREMENT,
  skill_id INTEGER NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
  run_id   TEXT    REFERENCES runs(id) ON DELETE SET NULL,
  used_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_skill_usage_skill  ON skill_usage(skill_id, used_at DESC);
CREATE INDEX IF NOT EXISTS idx_skill_usage_recent ON skill_usage(used_at DESC);

-- 12. SCHEDULED EVENTS
CREATE TABLE IF NOT EXISTS scheduled_events (
  id          TEXT PRIMARY KEY,
  type        TEXT NOT NULL,
  text        TEXT NOT NULL,
  at          INTEGER,
  schedule    TEXT,
  topic_id    TEXT REFERENCES topics(id) ON DELETE SET NULL,
  last_run_at INTEGER,
  created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_type ON scheduled_events(type);
CREATE INDEX IF NOT EXISTS idx_events_at   ON scheduled_events(at);

-- 13. TOPIC SETTINGS
CREATE TABLE IF NOT EXISTS topic_settings (
  topic_id    TEXT PRIMARY KEY REFERENCES topics(id) ON DELETE CASCADE,
  model_tier0 TEXT,
  model_tier1 TEXT,
  model_tier2 TEXT
);

-- 14. TOPIC SUMMARIES
CREATE TABLE IF NOT EXISTS topic_summaries (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  topic_id   TEXT    NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
  date_key   TEXT    NOT NULL,
  content    TEXT    NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  UNIQUE(topic_id, date_key)
);
CREATE INDEX IF NOT EXISTS idx_topic_summaries ON topic_summaries(topic_id, date_key DESC);

-- 15. PERIOD SUMMARIES
CREATE TABLE IF NOT EXISTS period_summaries (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  level      TEXT NOT NULL,
  period_key TEXT NOT NULL,
  content    TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  UNIQUE(level, period_key)
);

-- 16. LLM USAGE
CREATE TABLE IF NOT EXISTS llm_usage (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id         TEXT REFERENCES runs(id) ON DELETE SET NULL,
  topic_id       TEXT REFERENCES topics(id) ON DELETE SET NULL,
  model          TEXT,
  source         TEXT,
  status         TEXT    NOT NULL DEFAULT 'pending',
  total_tokens   INTEGER,
  cost_total     REAL,
  request_at     INTEGER,
  first_token_at INTEGER,
  end_at         INTEGER,
  tokens         TEXT,
  cost           TEXT,
  meta           TEXT,
  error          TEXT
);
CREATE INDEX IF NOT EXISTS idx_llm_usage_run    ON llm_usage(run_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_topic  ON llm_usage(topic_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_req_at ON llm_usage(request_at DESC);
CREATE INDEX IF NOT EXISTS idx_llm_usage_source ON llm_usage(source);
