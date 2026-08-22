package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPalaceMemoryStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "palace_mem_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_memory.sqlite")
	store, err := NewMemoryStore(dbPath, 384, 30.0)
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer store.Close()

	// 1. Add memories into different rooms
	id1, err := store.AddPalaceMemory("main_repo", "preferences", "preference", "User prefers Go for backend daemons.", "test", nil)
	if err != nil {
		t.Fatalf("Failed to add memory 1: %v", err)
	}
	if id1 == "" {
		t.Fatal("Expected non-empty memory ID")
	}

	id2, err := store.AddPalaceMemory("main_repo", "architecture", "decision", "Selected SQLite FTS5 for hybrid RAG search.", "test", nil)
	if err != nil {
		t.Fatalf("Failed to add memory 2: %v", err)
	}
	if id2 == "" {
		t.Fatal("Expected non-empty memory ID")
	}

	// 2. Search scoped by room
	results, err := store.SearchMemories("backend daemons", "preferences", 5, 0.1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Expected at least 1 match in preferences room")
	}
	if results[0].Room != "preferences" {
		t.Fatalf("Expected room 'preferences', got '%s'", results[0].Room)
	}

	// 3. Verify Stats
	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	if stats.TotalMemories != 2 {
		t.Fatalf("Expected 2 total memories, got %d", stats.TotalMemories)
	}
	if stats.Rooms["preferences"] != 1 || stats.Rooms["architecture"] != 1 {
		t.Fatalf("Room counts mismatch: %+v", stats.Rooms)
	}
}
