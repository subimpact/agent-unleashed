package memory

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
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
	Category         string                 `json:"category"`
	Content          string                 `json:"content"`
	Source           string                 `json:"source"`
	CreatedAt        string                 `json:"created_at"`
	AccessCount      int                    `json:"access_count"`
	LastAccessed     string                 `json:"last_accessed,omitempty"`
	Similarity       float64                `json:"similarity"`
	VectorSimilarity float64                `json:"vector_similarity"`
	FTSBoost         float64                `json:"fts_boost"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type MemoryStore struct {
	dbPath    string
	vectorDim int
	db        *sql.DB
	mu        sync.RWMutex
}

func NewMemoryStore(dbPath string, vectorDim int) (*MemoryStore, error) {
	if vectorDim <= 0 {
		vectorDim = 384
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	store := &MemoryStore{
		dbPath:    dbPath,
		vectorDim: vectorDim,
		db:        db,
	}

	if err := store.initDB(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *MemoryStore) initDB() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		category TEXT NOT NULL,
		content TEXT NOT NULL,
		source TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		access_count INTEGER DEFAULT 0,
		last_accessed TIMESTAMP,
		embedding BLOB,
		metadata_json TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_memories_category ON memories(category);
	`
	if _, err := s.db.Exec(createTableSQL); err != nil {
		return err
	}

	// Try initializing FTS5 table
	_, _ = s.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			id UNINDEXED,
			content,
			category
		);
	`)

	return nil
}

func (s *MemoryStore) ComputeEmbedding(text string) []float32 {
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

func (s *MemoryStore) AddMemory(category, content, source string, metadata map[string]interface{}) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idBytes := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", category, content, time.Now().UnixNano())))
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
		INSERT OR REPLACE INTO memories (id, category, content, source, embedding, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	if _, err := s.db.Exec(query, memID, category, content, source, buf.Bytes(), string(metaJSON)); err != nil {
		return "", err
	}

	_, _ = s.db.Exec("DELETE FROM memories_fts WHERE id = ?", memID)
	_, _ = s.db.Exec("INSERT INTO memories_fts (id, content, category) VALUES (?, ?, ?)", memID, content, category)

	return memID, nil
}

func (s *MemoryStore) SearchMemories(query string, category string, limit int, minSimilarity float64) ([]Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryVec := s.ComputeEmbedding(query)
	ftsBoosts := make(map[string]float64)

	// FTS5 lookup
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
						ftsBoosts[id] = math.Min(0.3, math.Abs(rank)*0.05)
					}
				}
			}
		}
	}

	var sqlQuery string
	var args []interface{}
	if category != "" {
		sqlQuery = "SELECT id, category, content, source, created_at, embedding, metadata_json FROM memories WHERE category = ?"
		args = append(args, category)
	} else {
		sqlQuery = "SELECT id, category, content, source, created_at, embedding, metadata_json FROM memories"
	}

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Memory
	for rows.Next() {
		var m Memory
		var embBytes []byte
		var metaStr sql.NullString

		if err := rows.Scan(&m.ID, &m.Category, &m.Content, &m.Source, &m.CreatedAt, &embBytes, &metaStr); err != nil {
			continue
		}

		if len(embBytes) < s.vectorDim*4 {
			continue
		}

		var vecSim float32
		for i := 0; i < s.vectorDim; i++ {
			bits := binary.LittleEndian.Uint32(embBytes[i*4 : (i+1)*4])
			val := math.Float32frombits(bits)
			vecSim += queryVec[i] * val
		}

		boost := ftsBoosts[m.ID]
		finalScore := float64(vecSim) + boost

		if finalScore >= minSimilarity {
			m.Similarity = math.Round(finalScore*10000) / 10000
			m.VectorSimilarity = math.Round(float64(vecSim)*10000) / 10000
			m.FTSBoost = math.Round(boost*10000) / 10000

			if metaStr.Valid && metaStr.String != "" {
				_ = json.Unmarshal([]byte(metaStr.String), &m.Metadata)
			}
			results = append(results, m)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *MemoryStore) GetRecentMemories(limit int) ([]Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.Query("SELECT id, category, content, source, created_at FROM memories ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Category, &m.Content, &m.Source, &m.CreatedAt); err == nil {
			list = append(list, m)
		}
	}
	return list, nil
}

func (s *MemoryStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
