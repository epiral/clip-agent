package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"
)

// 子命令列表
var commandNames = []string{
	"create-topic",
	"list-topics",
	"get-runs",
	"send-message",
	"event-check",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: agent <command>")
		os.Exit(1)
	}

	if os.Args[1] == "--commands" {
		for _, name := range commandNames {
			fmt.Println(name)
		}
		return
	}

	// 初始化数据库
	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	subCmd := os.Args[1]
	switch subCmd {
	case "create-topic":
		cmdCreateTopic(db)
	case "list-topics":
		cmdListTopics(db)
	case "get-runs":
		cmdGetRuns(db)
	case "send-message":
		cmdSendMessage(db)
	case "event-check":
		cmdEventCheck()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", subCmd)
		os.Exit(1)
	}
}

// ---------- 数据库初始化 ----------

func openDB() (*sql.DB, error) {
	// 确定 data 目录
	workdir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	dataDir := filepath.Join(workdir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "agent.db")

	// 注册带 sqlite-vec 扩展的 driver
	vecDylibPath := findVecDylib(workdir)
	driverName := "sqlite3_with_vec"

	// 检查 driver 是否已注册
	registered := false
	for _, d := range sql.Drivers() {
		if d == driverName {
			registered = true
			break
		}
	}
	if !registered {
		sql.Register(driverName, &sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				if vecDylibPath != "" {
					// 加载 sqlite-vec 扩展
					if err := conn.LoadExtension(vecDylibPath, "sqlite3_vec_init"); err != nil {
						// 不致命，继续运行
						fmt.Fprintf(os.Stderr, "warning: failed to load vec0: %v\n", err)
					}
				}
				return nil
			},
		})
	}

	db, err := sql.Open(driverName, dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}

	// 初始化 schema
	if err := initSchema(db, workdir); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return db, nil
}

func findVecDylib(workdir string) string {
	// 优先从 lib/ 加载
	libPath := filepath.Join(workdir, "lib", "vec0.dylib")
	if _, err := os.Stat(libPath); err == nil {
		return libPath
	}
	// fallback: /tmp/gotest2/vec0.dylib
	fallback := "/tmp/gotest2/vec0.dylib"
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	return ""
}

func initSchema(db *sql.DB, workdir string) error {
	schemaPath := filepath.Join(workdir, "seed", "schema.sql")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema.sql: %w", err)
	}

	// 逐条执行 SQL（PRAGMA + CREATE TABLE）
	stmts := strings.Split(string(data), ";")
	for _, stmt := range stmts {
		// 去除注释行，保留实际 SQL
		var lines []string
		for _, line := range strings.Split(stmt, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			lines = append(lines, line)
		}
		cleaned := strings.TrimSpace(strings.Join(lines, "\n"))
		if cleaned == "" {
			continue
		}
		if _, err := db.Exec(cleaned); err != nil {
			// 跳过 vec0 虚拟表错误（如果扩展未加载）
			if strings.Contains(cleaned, "vec0") && strings.Contains(err.Error(), "no such module") {
				continue
			}
			return fmt.Errorf("exec sql: %w\nstatement: %s", err, cleaned)
		}
	}
	return nil
}

// ---------- 工具函数 ----------

func readStdin() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

func nowUnix() int64 {
	return time.Now().Unix()
}

func newUUID() string {
	return uuid.New().String()
}

// ---------- create-topic ----------

type createTopicInput struct {
	Name string `json:"name"`
}

type createTopicOutput struct {
	TopicID string `json:"topic_id"`
	Name    string `json:"name"`
}

func cmdCreateTopic(db *sql.DB) {
	data, err := readStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(1)
	}

	var input createTopicInput
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "parse json: %v\n", err)
		os.Exit(1)
	}

	if input.Name == "" {
		fmt.Fprintln(os.Stderr, "name is required")
		os.Exit(1)
	}

	id := newUUID()
	now := nowUnix()

	_, err = db.Exec(
		"INSERT INTO topics (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)",
		id, input.Name, now, now,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert topic: %v\n", err)
		os.Exit(1)
	}

	writeJSON(createTopicOutput{TopicID: id, Name: input.Name})
}

// ---------- list-topics ----------

type topicItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
}

func cmdListTopics(db *sql.DB) {
	rows, err := db.Query(
		"SELECT id, name, created_at FROM topics WHERE archived_at IS NULL ORDER BY created_at DESC",
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query topics: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var topics []topicItem
	for rows.Next() {
		var t topicItem
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			fmt.Fprintf(os.Stderr, "scan topic: %v\n", err)
			os.Exit(1)
		}
		topics = append(topics, t)
	}

	if topics == nil {
		topics = []topicItem{}
	}
	writeJSON(topics)
}

// ---------- get-runs ----------

type getRunsInput struct {
	TopicID string `json:"topic_id"`
}

type runItem struct {
	ID          string  `json:"id"`
	UserMessage *string `json:"user_message"`
	Summary     *string `json:"summary"`
	Status      string  `json:"status"`
	CreatedAt   int64   `json:"created_at"`
}

func cmdGetRuns(db *sql.DB) {
	data, err := readStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(1)
	}

	var input getRunsInput
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "parse json: %v\n", err)
		os.Exit(1)
	}

	rows, err := db.Query(
		"SELECT id, user_message, summary, status, created_at FROM runs WHERE topic_id = ? ORDER BY created_at ASC",
		input.TopicID,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query runs: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var runs []runItem
	for rows.Next() {
		var r runItem
		if err := rows.Scan(&r.ID, &r.UserMessage, &r.Summary, &r.Status, &r.CreatedAt); err != nil {
			fmt.Fprintf(os.Stderr, "scan run: %v\n", err)
			os.Exit(1)
		}
		runs = append(runs, r)
	}

	if runs == nil {
		runs = []runItem{}
	}
	writeJSON(runs)
}

// ---------- send-message ----------

type sendMessageInput struct {
	TopicID string `json:"topic_id"`
	Message string `json:"message"`
}

func cmdSendMessage(db *sql.DB) {
	data, err := readStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(1)
	}

	var input sendMessageInput
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "parse json: %v\n", err)
		os.Exit(1)
	}

	if input.TopicID == "" || input.Message == "" {
		fmt.Fprintln(os.Stderr, "topic_id and message are required")
		os.Exit(1)
	}

	// 获取 API key
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENROUTER_API_KEY not set")
		os.Exit(1)
	}

	// 查找上一条 run（最新的）
	var prevRunID *string
	err = db.QueryRow(
		"SELECT id FROM runs WHERE topic_id = ? ORDER BY created_at DESC LIMIT 1",
		input.TopicID,
	).Scan(&prevRunID)
	if err != nil && err != sql.ErrNoRows {
		fmt.Fprintf(os.Stderr, "query prev run: %v\n", err)
		os.Exit(1)
	}

	// 创建 run
	runID := newUUID()
	now := nowUnix()
	_, err = db.Exec(
		"INSERT INTO runs (id, topic_id, prev_run_id, user_message, status, started_at, created_at) VALUES (?, ?, ?, ?, 'in_progress', ?, ?)",
		runID, input.TopicID, prevRunID, input.Message, now, now,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert run: %v\n", err)
		os.Exit(1)
	}

	// 构建历史上下文：最近 10 条 completed runs 的 summary
	rows, err := db.Query(
		"SELECT user_message, summary FROM runs WHERE topic_id = ? AND status = 'completed' ORDER BY created_at DESC LIMIT 10",
		input.TopicID,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query history: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var historyParts []string
	for rows.Next() {
		var userMsg, summary *string
		if err := rows.Scan(&userMsg, &summary); err != nil {
			continue
		}
		part := ""
		if userMsg != nil {
			part += "User: " + *userMsg
		}
		if summary != nil {
			part += "\nAssistant: " + *summary
		}
		historyParts = append(historyParts, part)
	}

	// 反转历史（从旧到新）
	for i, j := 0, len(historyParts)-1; i < j; i, j = i+1, j-1 {
		historyParts[i], historyParts[j] = historyParts[j], historyParts[i]
	}

	systemPrompt := "你是 Epiral Agent，一个智能助手。请用中文回答。"
	if len(historyParts) > 0 {
		systemPrompt += "\n\n以下是之前的对话摘要：\n" + strings.Join(historyParts, "\n---\n")
	}

	// 构建 OpenRouter 请求
	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": input.Message},
	}

	reqBody := map[string]any{
		"model":    "google/gemini-2.5-flash",
		"messages": messages,
		"stream":   true,
	}

	reqJSON, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", strings.NewReader(string(reqJSON)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "create request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "API error %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	// 流式读取 SSE
	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	// 增大 buffer 以处理大 chunk
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			fullResponse.WriteString(content)
			// 实时输出到 stdout
			os.Stdout.WriteString(content)
		}
	}

	// 生成 summary（取前 200 字）
	responseText := fullResponse.String()
	summary := responseText
	summaryRunes := []rune(summary)
	if len(summaryRunes) > 200 {
		summary = string(summaryRunes[:200]) + "..."
	}

	// 保存 messages 为 JSON
	savedMessages := []map[string]string{
		{"role": "user", "content": input.Message},
		{"role": "assistant", "content": responseText},
	}
	messagesJSON, _ := json.Marshal(savedMessages)

	// 更新 run
	endedAt := nowUnix()
	_, err = db.Exec(
		"UPDATE runs SET status = 'completed', messages = ?, summary = ?, ended_at = ? WHERE id = ?",
		string(messagesJSON), summary, endedAt, runID,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nupdate run: %v\n", err)
		os.Exit(1)
	}
}

// ---------- event-check ----------

func cmdEventCheck() {
	fmt.Println("no events")
}
