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
	// Pinix host protocol
	"list-runs",
	"get-run",
	"send",
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
	// Pinix host protocol commands
	case "list-runs":
		cmdHostListRuns(db)
	case "get-run":
		cmdHostGetRun(db)
	case "send":
		cmdHostSend(db)
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
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "iterate topics: %v\n", err)
		os.Exit(1)
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
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "iterate runs: %v\n", err)
		os.Exit(1)
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

	cmdSendCore(db, input.TopicID, input.Message, "")
}

// ---------- event-check ----------

func cmdEventCheck() {
	fmt.Println("no events")
}

// ========== Pinix Host Protocol ==========
// The Pinix desktop host UI calls: list-runs, get-run, send
// These adapt the topic/run model to the host's flat conversation model.

// ---------- list-runs (host) ----------
// Returns latest run from each topic as a sidebar entry.

type hostRunItem struct {
	ID        string `json:"id"`
	Trigger   string `json:"trigger"`
	CreatedAt int64  `json:"created_at"`
}

func cmdHostListRuns(db *sql.DB) {
	// Get the latest run per topic (each topic = one sidebar conversation)
	rows, err := db.Query(`
		SELECT r.id, r.created_at
		FROM runs r
		INNER JOIN (
			SELECT topic_id, MAX(created_at) as max_created
			FROM runs
			GROUP BY topic_id
		) latest ON r.topic_id = latest.topic_id AND r.created_at = latest.max_created
		ORDER BY r.created_at DESC
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query runs: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var items []hostRunItem
	for rows.Next() {
		var item hostRunItem
		if err := rows.Scan(&item.ID, &item.CreatedAt); err != nil {
			fmt.Fprintf(os.Stderr, "scan run: %v\n", err)
			os.Exit(1)
		}
		item.Trigger = "chat"
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "iterate runs: %v\n", err)
		os.Exit(1)
	}

	if items == nil {
		items = []hostRunItem{}
	}
	writeJSON(items)
}

// ---------- get-run (host) ----------
// Given a run ID, find its topic and return all messages in order.

type hostGetRunInput struct {
	ID string `json:"id"`
}

type hostMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type hostGetRunOutput struct {
	Messages []hostMessage `json:"messages"`
}

func cmdHostGetRun(db *sql.DB) {
	data, err := readStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(1)
	}

	var input hostGetRunInput
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "parse json: %v\n", err)
		os.Exit(1)
	}

	// Find topic_id from the run
	var topicID string
	err = db.QueryRow("SELECT topic_id FROM runs WHERE id = ?", input.ID).Scan(&topicID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find run: %v\n", err)
		os.Exit(1)
	}

	// Get all runs in this topic, ordered by creation time
	rows, err := db.Query(
		"SELECT user_message, messages, summary, status FROM runs WHERE topic_id = ? ORDER BY created_at ASC",
		topicID,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query runs: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var allMessages []hostMessage
	for rows.Next() {
		var userMsg, msgsJSON, summary *string
		var status string
		if err := rows.Scan(&userMsg, &msgsJSON, &summary, &status); err != nil {
			continue
		}
		// Try to parse stored messages first
		if msgsJSON != nil && *msgsJSON != "" {
			var msgs []hostMessage
			if json.Unmarshal([]byte(*msgsJSON), &msgs) == nil {
				allMessages = append(allMessages, msgs...)
				continue
			}
		}
		// Fallback: reconstruct from user_message + summary
		if userMsg != nil {
			allMessages = append(allMessages, hostMessage{Role: "user", Content: *userMsg})
		}
		if summary != nil && status == "completed" {
			allMessages = append(allMessages, hostMessage{Role: "assistant", Content: *summary})
		}
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "iterate runs: %v\n", err)
		os.Exit(1)
	}

	writeJSON(hostGetRunOutput{Messages: allMessages})
}

// ---------- send (host) ----------
// Creates a run, streams LLM response, appends RUN_ID:<id> at end.

type hostSendInput struct {
	Message   string `json:"message"`
	PrevRunID string `json:"prev_run_id"`
	Trigger   string `json:"trigger"`
}

func cmdHostSend(db *sql.DB) {
	data, err := readStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(1)
	}

	var input hostSendInput
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "parse json: %v\n", err)
		os.Exit(1)
	}

	if input.Message == "" {
		fmt.Fprintln(os.Stderr, "message is required")
		os.Exit(1)
	}

	// Determine topic: find from prev_run_id or create new
	var topicID string
	if input.PrevRunID != "" {
		err = db.QueryRow("SELECT topic_id FROM runs WHERE id = ?", input.PrevRunID).Scan(&topicID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "find prev run topic: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Create new topic (auto-named from first few chars of message)
		topicID = newUUID()
		now := nowUnix()
		topicName := string([]rune(input.Message)[:min(len([]rune(input.Message)), 30)])
		_, err = db.Exec(
			"INSERT INTO topics (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)",
			topicID, topicName, now, now,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create topic: %v\n", err)
			os.Exit(1)
		}
	}

	// Reuse send-message logic with the resolved topicID
	// Set up as sendMessageInput and call the core logic
	runID := cmdSendCore(db, topicID, input.Message, input.PrevRunID)

	// Append RUN_ID marker for host UI
	os.Stdout.WriteString("RUN_ID:" + runID)
}

// cmdSendCore is the shared send logic used by both send-message and send (host).
// Returns the run ID.
func cmdSendCore(db *sql.DB, topicID, message, prevRunIDStr string) string {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		// Fallback: read from pinix secrets
		home, _ := os.UserHomeDir()
		if data, err := os.ReadFile(filepath.Join(home, ".config", "pinix", "secrets", "openrouter-api-key")); err == nil {
			apiKey = strings.TrimSpace(string(data))
		}
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENROUTER_API_KEY not set")
		os.Exit(1)
	}

	var prevRunID *string
	if prevRunIDStr != "" {
		prevRunID = &prevRunIDStr
	} else {
		// Find latest run in topic
		var latest string
		err := db.QueryRow(
			"SELECT id FROM runs WHERE topic_id = ? ORDER BY created_at DESC LIMIT 1",
			topicID,
		).Scan(&latest)
		if err == nil {
			prevRunID = &latest
		}
	}

	runID := newUUID()
	now := nowUnix()
	_, err := db.Exec(
		"INSERT INTO runs (id, topic_id, prev_run_id, user_message, status, started_at, created_at) VALUES (?, ?, ?, ?, 'in_progress', ?, ?)",
		runID, topicID, prevRunID, message, now, now,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert run: %v\n", err)
		os.Exit(1)
	}

	// Build history context
	rows, err := db.Query(
		"SELECT user_message, summary FROM runs WHERE topic_id = ? AND status = 'completed' ORDER BY created_at DESC LIMIT 10",
		topicID,
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

	// Reverse (oldest first)
	for i, j := 0, len(historyParts)-1; i < j; i, j = i+1, j-1 {
		historyParts[i], historyParts[j] = historyParts[j], historyParts[i]
	}

	systemPrompt := "你是 Epiral Agent，一个智能助手。请用中文回答。"
	if len(historyParts) > 0 {
		systemPrompt += "\n\n以下是之前的对话摘要：\n" + strings.Join(historyParts, "\n---\n")
	}

	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": message},
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

	// Stream SSE
	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
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
			os.Stdout.WriteString(content)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "\nstream read error: %v\n", err)
		db.Exec("UPDATE runs SET status = 'failed', ended_at = ? WHERE id = ?", nowUnix(), runID)
		os.Exit(1)
	}

	// Save
	responseText := fullResponse.String()
	summary := responseText
	summaryRunes := []rune(summary)
	if len(summaryRunes) > 200 {
		summary = string(summaryRunes[:200]) + "..."
	}

	savedMessages := []map[string]string{
		{"role": "user", "content": message},
		{"role": "assistant", "content": responseText},
	}
	messagesJSON, _ := json.Marshal(savedMessages)

	endedAt := nowUnix()
	_, err = db.Exec(
		"UPDATE runs SET status = 'completed', messages = ?, summary = ?, ended_at = ? WHERE id = ?",
		string(messagesJSON), summary, endedAt, runID,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nupdate run: %v\n", err)
		os.Exit(1)
	}

	return runID
}
