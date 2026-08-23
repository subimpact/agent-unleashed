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
	if _, err := e.db.Exec(schema); err != nil {
		return err
	}

	// Without this column CompressIfExceeds re-summarised the same oldest ten
	// messages on every turn, so summary nodes multiplied without bound and the
	// session token total never fell back under the threshold.
	_, _ = e.db.Exec("ALTER TABLE lcm_messages ADD COLUMN summarized INTEGER NOT NULL DEFAULT 0")
	_, _ = e.db.Exec("CREATE INDEX IF NOT EXISTS idx_lcm_pending ON lcm_messages(session_id, summarized)")
	return nil
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

// BuildContext renders the session's history for injection into the next
// prompt: every hierarchical summary node first, then as many recent verbatim
// messages as the token budget allows.
//
// Until this existed the LCM store was write-only. Messages and summaries
// accumulated in SQLite and nothing was ever fed back, so "Lossless Context
// Management" contributed nothing to what the model actually saw.
func (e *LCMEngine) BuildContext(sessionID string, tokenBudget int) (string, error) {
	if e == nil || e.db == nil {
		return "", nil
	}
	if tokenBudget <= 0 {
		tokenBudget = 2000
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var sections []string

	// Compressed history, oldest first.
	sumRows, err := e.db.Query(
		`SELECT id, depth, summary_text FROM lcm_summaries WHERE session_id = ? ORDER BY id ASC`, sessionID)
	if err == nil {
		var lines []string
		for sumRows.Next() {
			var id int64
			var depth int
			var text string
			if err := sumRows.Scan(&id, &depth, &text); err == nil {
				lines = append(lines, fmt.Sprintf("- [node #%d, depth %d] %s", id, depth, text))
			}
		}
		sumRows.Close()
		if len(lines) > 0 {
			sections = append(sections,
				"Compressed earlier history (expand any node verbatim with `:lcm expand <message_id>`):\n"+strings.Join(lines, "\n"))
		}
	}

	// Recent verbatim turns, newest first so the budget keeps the freshest.
	msgRows, err := e.db.Query(
		`SELECT id, role, content, tokens FROM lcm_messages WHERE session_id = ? AND summarized = 0 ORDER BY id DESC LIMIT 200`, sessionID)
	if err != nil {
		if len(sections) == 0 {
			return "", err
		}
		return "[Lossless Context Management]:\n" + strings.Join(sections, "\n\n"), nil
	}
	defer msgRows.Close()

	var recent []string
	spent := 0
	for msgRows.Next() {
		var id int64
		var role, content string
		var tokens int
		if err := msgRows.Scan(&id, &role, &content, &tokens); err != nil {
			continue
		}
		if spent+tokens > tokenBudget && len(recent) > 0 {
			break
		}
		spent += tokens
		recent = append(recent, fmt.Sprintf("[#%d %s] %s", id, role, content))
	}

	if len(recent) > 0 {
		// Reverse into chronological order.
		for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
			recent[i], recent[j] = recent[j], recent[i]
		}
		sections = append(sections, "Recent conversation:\n"+strings.Join(recent, "\n"))
	}

	if len(sections) == 0 {
		return "", nil
	}
	return "[Lossless Context Management]:\n" + strings.Join(sections, "\n\n"), nil
}

// PendingTokens is the number of tokens still held as uncompressed verbatim
// messages for a session.
func (e *LCMEngine) PendingTokens(sessionID string) int {
	if e == nil || e.db == nil {
		return 0
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	var total int
	_ = e.db.QueryRow(`SELECT COALESCE(SUM(tokens), 0) FROM lcm_messages WHERE session_id = ? AND summarized = 0`, sessionID).Scan(&total)
	return total
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
	var liveTokens int
	_ = e.db.QueryRow(`SELECT COALESCE(SUM(tokens), 0) FROM lcm_messages WHERE session_id = ? AND summarized = 0`, sessionID).Scan(&liveTokens)
	sb.WriteString(fmt.Sprintf("- Total Stored Tokens: %d (%d still verbatim, %d folded into summaries)\n", totalTokens, liveTokens, totalTokens-liveTokens))
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

	// Only live (unsummarised) messages count towards the threshold, otherwise
	// a session that crossed it once would compress on every single turn.
	var totalTokens int
	_ = e.db.QueryRow(`SELECT COALESCE(SUM(tokens), 0) FROM lcm_messages WHERE session_id = ? AND summarized = 0`, sessionID).Scan(&totalTokens)
	if totalTokens < tokenThreshold {
		return nil, nil // No compression needed
	}

	// Retrieve the oldest chunk that has not been folded into a summary yet.
	rows, err := e.db.Query(`SELECT id, role, content FROM lcm_messages WHERE session_id = ? AND summarized = 0 ORDER BY id ASC LIMIT 10`, sessionID)
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

	// Mark the covered messages so the next pass moves forward. Nothing is
	// deleted - the verbatim rows stay addressable through Expand and Grep.
	markArgs := make([]interface{}, 0, len(ids)+1)
	markArgs = append(markArgs, sessionID)
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		markArgs = append(markArgs, id)
	}
	_, _ = e.db.Exec(
		`UPDATE lcm_messages SET summarized = 1 WHERE session_id = ? AND id IN (`+strings.Join(placeholders, ",")+`)`,
		markArgs...,
	)

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
