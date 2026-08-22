package cron

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestCronEngine(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cron_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "cron_test.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open sqlite: %v", err)
	}
	defer db.Close()

	ce, err := NewCronEngine(db, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create cron engine: %v", err)
	}

	// 1. Add job
	job, err := ce.AddJob("*/5 * * * *", "Summarize daily progress", "log", "")
	if err != nil {
		t.Fatalf("Failed to add cron job: %v", err)
	}
	if job.ID == "" {
		t.Fatal("Expected valid job ID")
	}

	// 2. List jobs
	jobs := ce.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(jobs))
	}

	// 3. Match expression
	now := time.Date(2026, 8, 23, 10, 15, 0, 0, time.UTC)
	if !matchesCron("*/5 * * * *", now) {
		t.Fatal("Expected */5 to match minute 15")
	}

	// 4. Remove job
	if err := ce.RemoveJob(job.ID); err != nil {
		t.Fatalf("Failed to remove job: %v", err)
	}
	if len(ce.ListJobs()) != 0 {
		t.Fatal("Expected 0 jobs after removal")
	}
}
