package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToolRunnerFileOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tools_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runner := NewToolRunner(tempDir, nil)

	// 1. Write file
	writeRes := runner.WriteFile("test_folder/sample.txt", "Hello Agent-Unleashed")
	if writeRes.Error != "" {
		t.Fatalf("WriteFile failed: %s", writeRes.Error)
	}

	// 2. View file
	viewRes := runner.ViewFile("test_folder/sample.txt")
	if viewRes.Error != "" {
		t.Fatalf("ViewFile failed: %s", viewRes.Error)
	}
	if viewRes.Output != "Hello Agent-Unleashed" {
		t.Fatalf("Expected content 'Hello Agent-Unleashed', got '%s'", viewRes.Output)
	}

	// 3. List directory
	listRes := runner.ListDir("test_folder")
	if listRes.Error != "" {
		t.Fatalf("ListDir failed: %s", listRes.Error)
	}
	if !filepath.IsAbs(runner.workspaceRoot) {
		t.Fatal("Expected absolute workspace root")
	}
}
