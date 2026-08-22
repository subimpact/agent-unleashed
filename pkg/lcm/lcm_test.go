package lcm

import (
	"os"
	"testing"
)

func TestLCMEngine(t *testing.T) {
	tempFile, err := os.CreateTemp("", "lcm_test_*.sqlite")
	if err != nil {
		t.Fatalf("CreateTemp error: %v", err)
	}
	dbPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(dbPath)

	engine, err := NewLCMEngine(dbPath)
	if err != nil {
		t.Fatalf("NewLCMEngine error: %v", err)
	}
	defer engine.Close()

	sessionID := "test-session-001"

	// 1. Append messages
	m1, err := engine.AppendMessage(sessionID, "user", "Implement Lossless Context Management DAG in Go")
	if err != nil {
		t.Fatalf("AppendMessage error: %v", err)
	}
	if m1.ID == 0 {
		t.Fatal("Expected non-zero message ID")
	}

	_, _ = engine.AppendMessage(sessionID, "assistant", "I will create pkg/lcm/ with modernc.org/sqlite support.")
	_, _ = engine.AppendMessage(sessionID, "user", "Please verify zero CGO and fast DAG indexing.")

	// 2. Grep
	matches, err := engine.Grep(sessionID, "modernc.org")
	if err != nil {
		t.Fatalf("Grep error: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("Expected at least 1 match for 'modernc.org'")
	}

	// 3. Describe
	desc, err := engine.Describe(sessionID)
	if err != nil {
		t.Fatalf("Describe error: %v", err)
	}
	if desc == "" {
		t.Fatal("Expected non-empty description")
	}

	// 4. Compress
	node, err := engine.CompressIfExceeds(sessionID, 5)
	if err != nil {
		t.Fatalf("CompressIfExceeds error: %v", err)
	}
	if node == nil {
		t.Fatal("Expected summary node creation")
	}

	// 5. Expand
	exp, err := engine.Expand(m1.ID)
	if err != nil {
		t.Fatalf("Expand error: %v", err)
	}
	if exp.Content != "Implement Lossless Context Management DAG in Go" {
		t.Fatalf("Unexpected expanded content: %s", exp.Content)
	}
}
