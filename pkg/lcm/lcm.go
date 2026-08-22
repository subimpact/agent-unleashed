package lcm

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type LCMMessage struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Tokens    int       `json:"tokens"`
	Timestamp time.Time `json:"timestamp"`
}

type SummaryNode struct {
	ID          int64     `json:"id"`
	SessionID   string    `json:"session_id"`
	Depth       int       `json:"depth"`
	SummaryText string    `json:"summary_text"`
	MessageIDs  string    `json:"message_ids"`
	Tokens      int       `json:"tokens"`
	CreatedAt   time.Time `json:"created_at"`
}

type LCMEngine struct {
	dbPath string
	db     *sql.DB
	mu     sync.RWMutex
}

func NewLCMEngine(dbPath string) (*LCMEngine, error) {
	if dbPath == "" {
		dbPath = "./data/lcm.sqlite"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open LCM sqlite db: %w", err)
	}

	engine := &LCMEngine{
		dbPath: dbPath,
		db:     db,
	}

	if err := engine.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init LCM schema: %w", err)
	}

	return engine, nil
}

func (e *LCMEngine) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS lcm_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		tokens INTEGER NOT NULL,
		timestamp DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_lcm_session ON lcm_messages(session_id);

	CREATE TABLE IF NOT EXISTS lcm_summaries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		depth INTEGER NOT NULL,
		summary_text TEXT NOT NULL,
		message_ids TEXT NOT NULL,
		tokens INTEGER NOT NULL,
		created_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_lcm_summaries_session ON lcm_summaries(session_id);
	`
	_, err := e.db.Exec(schema)
	return err
}

func (e *LCMEngine) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

func (e *LCMEngine) AppendMessage(sessionID, role, content string) (*LCMMessage, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	tokens := int(float64(len(strings.Fields(content))) * 1.33)
	if tokens < 1 {
		tokens = 1
	}

	now := time.Now()
	res, err := e.db.Exec(
		`INSERT INTO lcm_messages (session_id, role, content, tokens, timestamp) VALUES (?, ?, ?, ?, ?)`,
		sessionID, role, content, tokens, now,
	)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	return &LCMMessage{
		ID:        id,
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Tokens:    tokens,
		Timestamp: now,
	}, nil
}

func (e *LCMEngine) Grep(sessionID, query string) ([]*LCMMessage, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	q := "%" + query + "%"
	var rows *sql.Rows
	var err error

	if sessionID == "" || sessionID == "all" {
		rows, err = e.db.Query(`SELECT id, session_id, role, content, tokens, timestamp FROM lcm_messages WHERE content LIKE ? ORDER BY id DESC LIMIT 50`, q)
	} else {
		rows, err = e.db.Query(`SELECT id, session_id, role, content, tokens, timestamp FROM lcm_messages WHERE session_id = ? AND content LIKE ? ORDER BY id DESC LIMIT 50`, sessionID, q)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []*LCMMessage
	for rows.Next() {
		var m LCMMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Tokens, &m.Timestamp); err == nil {
			matches = append(matches, &m)
		}
	}
	return matches, nil
}

func (e *LCMEngine) Describe(sessionID string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var totalMessages int
	var totalTokens int
	_ = e.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(tokens), 0) FROM lcm_messages WHERE session_id = ?`, sessionID).Scan(&totalMessages, &totalTokens)

	var summaryCount int
	_ = e.db.QueryRow(`SELECT COUNT(*) FROM lcm_summaries WHERE session_id = ?`, sessionID).Scan(&summaryCount)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 Lossless Context Management (LCM) DAG Summary [Session: %s]:\n", sessionID))
	sb.WriteString(fmt.Sprintf("- Total Historical Messages Stored: %d\n", totalMessages))
	sb.WriteString(fmt.Sprintf("- Total Uncompressed Tokens: %d\n", totalTokens))
	sb.WriteString(fmt.Sprintf("- Hierarchical Summary DAG Nodes: %d\n\n", summaryCount))

	rows, err := e.db.Query(`SELECT id, depth, summary_text, tokens, created_at FROM lcm_summaries WHERE session_id = ? ORDER BY depth ASC, id DESC LIMIT 10`, sessionID)
	if err == nil {
		defer rows.Close()
		sb.WriteString("Hierarchy DAG Nodes:\n")
		for rows.Next() {
			var id int64
			var depth, tok int
			var summary string
			var createdAt time.Time
			if err := rows.Scan(&id, &depth, &summary, &tok, &createdAt); err == nil {
				sb.WriteString(fmt.Sprintf("  • [Node #%d | Depth %d | %d tok] %s\n", id, depth, tok, summary))
			}
		}
	}

	return sb.String(), nil
}

func (e *LCMEngine) Expand(messageID int64) (*LCMMessage, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var m LCMMessage
	err := e.db.QueryRow(`SELECT id, session_id, role, content, tokens, timestamp FROM lcm_messages WHERE id = ?`, messageID).
		Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Tokens, &m.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("message id %d not found in LCM store", messageID)
	}
	return &m, nil
}

func (e *LCMEngine) CompressIfExceeds(sessionID string, tokenThreshold int) (*SummaryNode, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var totalTokens int
	_ = e.db.QueryRow(`SELECT COALESCE(SUM(tokens), 0) FROM lcm_messages WHERE session_id = ?`, sessionID).Scan(&totalTokens)
	if totalTokens < tokenThreshold {
		return nil, nil // No compression needed
	}

	// Retrieve oldest unsummarized chunk of messages (e.g. oldest 10 messages)
	rows, err := e.db.Query(`SELECT id, role, content FROM lcm_messages WHERE session_id = ? ORDER BY id ASC LIMIT 10`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	var summaries []string
	for rows.Next() {
		var id int64
		var role, content string
		if err := rows.Scan(&id, &role, &content); err == nil {
			ids = append(ids, fmt.Sprintf("%d", id))
			firstLine := strings.Split(content, "\n")[0]
			if len(firstLine) > 80 {
				firstLine = firstLine[:77] + "..."
			}
			summaries = append(summaries, fmt.Sprintf("%s: %s", role, firstLine))
		}
	}

	if len(ids) == 0 {
		return nil, nil
	}

	summaryContent := strings.Join(summaries, " | ")
	summaryTokens := int(float64(len(strings.Fields(summaryContent))) * 1.33)
	now := time.Now()

	res, err := e.db.Exec(
		`INSERT INTO lcm_summaries (session_id, depth, summary_text, message_ids, tokens, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, 1, summaryContent, strings.Join(ids, ","), summaryTokens, now,
	)
	if err != nil {
		return nil, err
	}

	nodeID, _ := res.LastInsertId()
	return &SummaryNode{
		ID:          nodeID,
		SessionID:   sessionID,
		Depth:       1,
		SummaryText: summaryContent,
		MessageIDs:  strings.Join(ids, ","),
		Tokens:      summaryTokens,
		CreatedAt:   now,
	}, nil
}
