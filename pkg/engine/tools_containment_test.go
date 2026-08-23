package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The file tools join a caller-supplied path onto the workspace root. Now that
// they are reachable from the REPL, "../.." must not walk out of the workspace.
func TestToolRunnerContainment(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(base, "secret.txt")
	if err := os.WriteFile(secret, []byte("do not read me"), 0o600); err != nil {
		t.Fatal(err)
	}

	tr := NewToolRunner(workspace, nil)

	escapes := []string{
		"../secret.txt",
		"../../secret.txt",
		"sub/../../secret.txt",
		filepath.Join("..", "secret.txt"),
	}
	for _, p := range escapes {
		if res := tr.ViewFile(p); res.Error == "" {
			t.Errorf("ViewFile(%q) escaped the workspace and returned %q", p, res.Output)
		}
		if res := tr.WriteFile(p, "owned"); res.Error == "" {
			t.Errorf("WriteFile(%q) escaped the workspace", p)
		}
		if res := tr.ListDir(p); res.Error == "" {
			t.Errorf("ListDir(%q) escaped the workspace", p)
		}
	}

	if got, err := os.ReadFile(secret); err != nil || string(got) != "do not read me" {
		t.Errorf("the file outside the workspace was modified: %q, %v", got, err)
	}

	if res := tr.ViewFile(secret); res.Error == "" {
		t.Error("an absolute path was accepted")
	}
}

func TestToolRunnerAllowsWorkspacePaths(t *testing.T) {
	workspace := t.TempDir()
	tr := NewToolRunner(workspace, nil)

	if res := tr.WriteFile("notes/todo.md", "hello"); res.Error != "" {
		t.Fatalf("WriteFile in workspace: %s", res.Error)
	}
	if res := tr.ViewFile("notes/todo.md"); res.Error != "" || res.Output != "hello" {
		t.Errorf("ViewFile = (%q, %q)", res.Output, res.Error)
	}
	if res := tr.ListDir("notes"); res.Error != "" || !strings.Contains(res.Output, "todo.md") {
		t.Errorf("ListDir = (%q, %q)", res.Output, res.Error)
	}
	if res := tr.ListDir(""); res.Error != "" {
		t.Errorf("ListDir(\"\") = %q, want the workspace root listing", res.Error)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"git-release", "git-release"},
		{"My Skill", "my-skill"},
		{"../../evil", "evil"},
		{"a/b/c", "a-b-c"},
		{"...", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := SanitizeName(c.in); got != c.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSaveSkillCannotEscape(t *testing.T) {
	workspace := t.TempDir()
	tr := NewToolRunner(workspace, nil)

	res := tr.SaveSkill("../../pwned", "d", "body")
	if res.Error == "" {
		abs, _ := filepath.Abs(res.Output)
		if !strings.Contains(abs, workspace) && !strings.Contains(res.Output, workspace) {
			t.Errorf("SaveSkill wrote outside the workspace: %s", res.Output)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(workspace), "pwned")); err == nil {
		t.Error("SaveSkill created a directory outside the workspace")
	}
}
