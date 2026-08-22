package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillIndexer(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "skills_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	skillFolder := filepath.Join(tempDir, "git-release-workflow")
	_ = os.MkdirAll(skillFolder, 0755)

	content := `---
name: git-release
description: Automates creating GitHub releases and tagging commits.
---

# Git Release Workflow
1. Run git tag
2. Push to origin
`
	_ = os.WriteFile(filepath.Join(skillFolder, "SKILL.md"), []byte(content), 0644)

	indexer := NewSkillIndexer(tempDir)
	skills := indexer.ListSkills()
	if len(skills) != 1 {
		t.Fatalf("Expected 1 indexed skill, got %d", len(skills))
	}

	idx := indexer.GenerateLightweightIndex()
	if !strings.Contains(idx, "skill:git-release") {
		t.Fatalf("Expected lightweight index to contain 'skill:git-release', got: %s", idx)
	}

	full, err := indexer.GetFullSkillContent("git-release")
	if err != nil || !strings.Contains(full, "Git Release Workflow") {
		t.Fatalf("Failed to retrieve full skill content: %v", err)
	}
}
