package wiki

import (
	"os"
	"testing"
)

func TestWikiEngine(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wiki_test_*")
	if err != nil {
		t.Fatalf("TempDir error: %v", err)
	}
	defer os.RemoveAll(tempDir)

	we := NewWikiEngine(tempDir)
	if err := we.Init(); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	page, err := we.AddOrUpdatePage("auth", "Authentication Protocol", "security", "JWT and session tokens", "Full details on auth flow.", []string{"auth", "jwt"})
	if err != nil {
		t.Fatalf("AddPage error: %v", err)
	}
	if page.Slug != "auth" {
		t.Fatalf("Expected slug 'auth', got '%s'", page.Slug)
	}

	// Search
	matches := we.Search("JWT")
	if len(matches) == 0 {
		t.Fatal("Expected match for 'JWT'")
	}

	// Auto Ingest
	autoPage, err := we.AutoIngest("database", "SQLite Pure Go", "Using modernc.org/sqlite without CGO.")
	if err != nil {
		t.Fatalf("AutoIngest error: %v", err)
	}
	if autoPage.Slug != "database" {
		t.Fatalf("Expected slug 'database', got '%s'", autoPage.Slug)
	}

	index := we.GenerateLightweightIndex()
	if index == "" {
		t.Fatal("Expected non-empty index")
	}
}
