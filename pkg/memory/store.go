package memory

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Memory struct {
	ID               string                 `json:"id"`
	Wing             string                 `json:"wing"`    // Project / Workspace scope
	Room             string                 `json:"room"`    // Topic / Domain (e.g. 'preferences', 'architecture', 'api')
	Hall             string                 `json:"hall"`    // Category: 'fact', 'preference', 'lesson', 'decision'
	Content          string                 `json:"content"` // Verbatim content
	Source           string                 `json:"source"`  // Source adapter / channel
	CreatedAt        string                 `json:"created_at"`
	AccessCount      int                    `json:"access_count"`
	LastAccessed     string                 `json:"last_accessed,omitempty"`
	Similarity       float64                `json:"similarity"`
	VectorSimilarity float64                `json:"vector_similarity"`
	FTSBoost         float64                `json:"fts_boost"`
	DecayFactor      float64                `json:"decay_factor"`
	AccessBoost      float64                `json:"access_boost"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type MemoryStats struct {
	TotalMemories int            `json:"total_memories"`
	Rooms         map[string]int `json:"rooms"`
	Halls         map[string]int `json:"halls"`
	TopAccessed   []Memory       `json:"top_accessed"`
}

// errMemoryDisabled is returned by every store method when memory.enabled is
// false: the engine leaves MemoryStore nil, and callers should degrade rather
// than dereference it.
var errMemoryDisabled = errors.New("palace-mnemosyne memory is disabled")

type MemoryStore struct {
	dbPath            string
	vectorDim         int
	decayHalfLifeDays float64
	db                *sql.DB
	mu                sync.RWMutex
}

func NewMemoryStore(dbPath string, vectorDim int, decayHalfLifeDays float64) (*MemoryStore, error) {
	if vectorDim <= 0 {
		vectorDim = 384
	}
	if decayHalfLifeDays <= 0 {
		decayHalfLifeDays = 30.0
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	store := &MemoryStore{
		dbPath:            dbPath,
		vectorDim:         vectorDim,
		decayHalfLifeDays: decayHalfLifeDays,
		db:                db,
	}

	if err := store.initDB(); err != nil {
		return nil, err
	}

	return store, nil
}

// GetDB returns the underlying handle, or nil when memory is disabled and the
// store was never constructed. Callers must check before using it.
func (s *MemoryStore) GetDB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func (s *MemoryStore) initDB() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		wing TEXT NOT NULL DEFAULT 'default',
		room TEXT NOT NULL DEFAULT 'general',
		hall TEXT NOT NULL DEFAULT 'fact',
		content TEXT NOT NULL,
		source TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		access_count INTEGER DEFAULT 0,
		last_accessed TIMESTAMP,
		embedding BLOB,
		metadata_json TEXT
	);
	`
	if _, err := s.db.Exec(createTableSQL); err != nil {
		return err
	}

	// Schema migrations for existing databases if created previously
	_, _ = s.db.Exec("ALTER TABLE memories ADD COLUMN wing TEXT NOT NULL DEFAULT 'default';")
	_, _ = s.db.Exec("ALTER TABLE memories ADD COLUMN room TEXT NOT NULL DEFAULT 'general';")
	_, _ = s.db.Exec("ALTER TABLE memories ADD COLUMN hall TEXT NOT NULL DEFAULT 'fact';")

	// Create indexes after columns are guaranteed to exist
	_, _ = s.db.Exec("CREATE INDEX IF NOT EXISTS idx_memories_wing_room ON memories(wing, room);")
	_, _ = s.db.Exec("CREATE INDEX IF NOT EXISTS idx_memories_hall ON memories(hall);")

	// SQLite FTS5 Full-Text Search Table
	_, _ = s.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			id UNINDEXED,
			content,
			room,
			hall
		);
	`)

	return nil
}

func (s *MemoryStore) ComputeEmbedding(text string) []float32 {
	if s == nil {
		return nil
	}
	vec := make([]float32, s.vectorDim)
	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return vec
	}

	for _, word := range words {
		l := len(word)
		if l < 3 {
			h := md5.Sum([]byte(word))
			idx := int(binary.BigEndian.Uint32(h[:4])) % s.vectorDim
			vec[idx] += 1.0
			continue
		}
		for i := 0; i <= l-3; i++ {
			gram := word[i : i+3]
			h := md5.Sum([]byte(gram))
			idx := int(binary.BigEndian.Uint32(h[:4])) % s.vectorDim
			if (h[4] % 2) == 0 {
				vec[idx] += 1.0
			} else {
				vec[idx] -= 1.0
			}
		}
	}

	// Normalize
	var sumSq float64
	for _, val := range vec {
		sumSq += float64(val * val)
	}
	norm := math.Sqrt(sumSq)
	if norm > 0 {
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}

	return vec
}

func (s *MemoryStore) AddPalaceMemory(wing, room, hall, content, source string, metadata map[string]interface{}) (string, error) {
	if s == nil {
		return "", errMemoryDisabled
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if wing == "" {
		wing = "default"
	}
	if room == "" {
		room = "general"
	}
	if hall == "" {
		hall = "fact"
	}

	idBytes := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%s:%d", wing, room, hall, content, time.Now().UnixNano())))
	memID := hex.EncodeToString(idBytes[:8])

	vec := s.ComputeEmbedding(content)
	buf := new(bytes.Buffer)
	for _, f := range vec {
		if err := binary.Write(buf, binary.LittleEndian, f); err != nil {
			return "", err
		}
	}

	metaJSON, _ := json.Marshal(metadata)

	query := `
		INSERT OR REPLACE INTO memories (id, wing, room, hall, content, source, embedding, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := s.db.Exec(query, memID, wing, room, hall, content, source, buf.Bytes(), string(metaJSON)); err != nil {
		return "", err
	}

	_, _ = s.db.Exec("DELETE FROM memories_fts WHERE id = ?", memID)
	_, _ = s.db.Exec("INSERT INTO memories_fts (id, content, room, hall) VALUES (?, ?, ?, ?)", memID, content, room, hall)

	return memID, nil
}

func (s *MemoryStore) AddMemory(hall, content, source string, metadata map[string]interface{}) (string, error) {
	room := "general"
	if hall == "preference" {
		room = "preferences"
	} else if hall == "lesson" {
		room = "lessons"
	}
	return s.AddPalaceMemory("default", room, hall, content, source, metadata)
}

func (s *MemoryStore) SearchMemories(query string, roomFilter string, limit int, minSimilarity float64) ([]Memory, error) {
	if s == nil {
		return nil, errMemoryDisabled
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryVec := s.ComputeEmbedding(query)
	ftsBoosts := make(map[string]float64)

	// 1. FTS5 full-text search boost
	cleanWords := strings.Fields(query)
	if len(cleanWords) > 0 {
		var ftsParts []string
		for _, w := range cleanWords {
			clean := strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
					return r
				}
				return -1
			}, w)
			if len(clean) > 0 {
				ftsParts = append(ftsParts, clean)
			}
		}
		if len(ftsParts) > 0 {
			ftsQuery := strings.Join(ftsParts, " OR ")
			rows, err := s.db.Query("SELECT id, rank FROM memories_fts WHERE memories_fts MATCH ? ORDER BY rank LIMIT 10", ftsQuery)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id string
					var rank float64
					if err := rows.Scan(&id, &rank); err == nil {
						ftsBoosts[id] = math.Min(0.35, math.Abs(rank)*0.05)
					}
				}
			}
		}
	}

	// 2. Query candidates
	var sqlQuery string
	var args []interface{}
	if roomFilter != "" {
		sqlQuery = "SELECT id, wing, room, hall, content, source, created_at, access_count, embedding, metadata_json FROM memories WHERE room = ?"
		args = append(args, roomFilter)
	} else {
		sqlQuery = "SELECT id, wing, room, hall, content, source, created_at, access_count, embedding, metadata_json FROM memories"
	}

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	var results []Memory
	var matchedIDs []string

	for rows.Next() {
		var m Memory
		var embBytes []byte
		var metaStr sql.NullString
		var createdAtStr string

		if err := rows.Scan(&m.ID, &m.Wing, &m.Room, &m.Hall, &m.Content, &m.Source, &createdAtStr, &m.AccessCount, &embBytes, &metaStr); err != nil {
			continue
		}
		m.CreatedAt = createdAtStr

		if len(embBytes) < s.vectorDim*4 {
			continue
		}

		// Vector Cosine Similarity
		var vecSim float32
		for i := 0; i < s.vectorDim; i++ {
			bits := binary.LittleEndian.Uint32(embBytes[i*4 : (i+1)*4])
			val := math.Float32frombits(bits)
			vecSim += queryVec[i] * val
		}

		ftsBoost := ftsBoosts[m.ID]

		// Mnemosyne Ebbinghaus Cognitive Decay Calculation
		createdTime, parseErr := time.Parse("2006-01-02 15:04:05", createdAtStr)
		if parseErr != nil {
			createdTime = now
		}
		daysDiff := now.Sub(createdTime).Hours() / 24.0
		if daysDiff < 0 {
			daysDiff = 0
		}
		decayFactor := math.Exp(-0.693 * daysDiff / s.decayHalfLifeDays)
		m.DecayFactor = math.Round(decayFactor*1000) / 1000

		// Spaced Repetition Reinforcement Boost
		accessBoost := 1.0 + 0.1*math.Log(1.0+float64(m.AccessCount))
		m.AccessBoost = math.Round(accessBoost*1000) / 1000

		// Final Combined Cognitive Score
		finalScore := (float64(vecSim) + ftsBoost) * decayFactor * accessBoost

		if finalScore >= minSimilarity {
			m.Similarity = math.Round(finalScore*10000) / 10000
			m.VectorSimilarity = math.Round(float64(vecSim)*10000) / 10000
			m.FTSBoost = math.Round(ftsBoost*10000) / 10000

			if metaStr.Valid && metaStr.String != "" {
				_ = json.Unmarshal([]byte(metaStr.String), &m.Metadata)
			}
			results = append(results, m)
			matchedIDs = append(matchedIDs, m.ID)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	// Update access count asynchronously in transaction
	if len(matchedIDs) > 0 {
		go func(ids []string) {
			for _, id := range ids {
				_, _ = s.db.Exec("UPDATE memories SET access_count = access_count + 1, last_accessed = CURRENT_TIMESTAMP WHERE id = ?", id)
			}
		}(matchedIDs)
	}

	return results, nil
}

func (s *MemoryStore) GetStats() (*MemoryStats, error) {
	if s == nil {
		return nil, errMemoryDisabled
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &MemoryStats{
		Rooms: make(map[string]int),
		Halls: make(map[string]int),
	}

	// Total count
	_ = s.db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&stats.TotalMemories)

	// Room counts
	rRows, err := s.db.Query("SELECT room, COUNT(*) FROM memories GROUP BY room")
	if err == nil {
		defer rRows.Close()
		for rRows.Next() {
			var room string
			var count int
			if err := rRows.Scan(&room, &count); err == nil {
				stats.Rooms[room] = count
			}
		}
	}

	// Hall counts
	hRows, err := s.db.Query("SELECT hall, COUNT(*) FROM memories GROUP BY hall")
	if err == nil {
		defer hRows.Close()
		for hRows.Next() {
			var hall string
			var count int
			if err := hRows.Scan(&hall, &count); err == nil {
				stats.Halls[hall] = count
			}
		}
	}

	// Top accessed
	stats.TopAccessed, _ = s.GetRecentMemories(5)

	return stats, nil
}

func (s *MemoryStore) GetRecentMemories(limit int) ([]Memory, error) {
	if s == nil {
		return nil, errMemoryDisabled
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.Query("SELECT id, wing, room, hall, content, source, created_at, access_count FROM memories ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Wing, &m.Room, &m.Hall, &m.Content, &m.Source, &m.CreatedAt, &m.AccessCount); err == nil {
			list = append(list, m)
		}
	}
	return list, nil
}

func (s *MemoryStore) Close() error {
	if s == nil {
		return nil
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
