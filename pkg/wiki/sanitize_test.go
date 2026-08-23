package wiki

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeSlug(t *testing.T) {
	bs := string(rune(92)) // a single backslash, without a source escape

	cases := []struct{ in, want string }{
		{"architecture", "architecture"},
		{"Memory Palace", "memory_palace"},
		{"cli-adapters", "cli_adapters"},
		{"../escaped", "escaped"},
		{"../../../../pwned", "pwned"},
		{"..", ""},
		{"/", ""},
		{"C:" + bs + "Windows" + bs + "System32" + bs + "drivers", "c_windows_system32_drivers"},
		{"foo/../../bar", "foo_bar"},
		{".ssh", "ssh"},
		{"", ""},
		{"!!!", ""},
		{"a" + string(make([]byte, 0)) + "verylongtopicnamethatgoesonandonandon", "averylongtopicnamethatgoesonan"},
	}
	for _, c := range cases {
		if got := SanitizeSlug(c.in); got != c.want {
			t.Errorf("SanitizeSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// engine.Chat derives the auto-ingest topic from the first word of a user
// message, which arrives over unauthenticated channels. It must never escape
// the wiki directory.
func TestAutoIngestCannotEscapeWikiDir(t *testing.T) {
	base := t.TempDir()
	wikiDir := filepath.Join(base, "nested", "wiki")
	w := NewWikiEngine(wikiDir)

	for _, topic := range []string{"../escaped", "../../escaped", ".." + string(rune(92)) + "escaped", "/etc/passwd"} {
		page, err := w.AutoIngest(topic, "summary", "payload")
		if err != nil {
			continue // refused outright, also fine
		}
		abs, absErr := filepath.Abs(page.Path)
		if absErr != nil {
			t.Fatalf("Abs(%q): %v", page.Path, absErr)
		}
		wikiAbs, _ := filepath.Abs(wikiDir)
		if filepath.Dir(abs) != wikiAbs {
			t.Errorf("topic %q wrote to %s, outside %s", topic, abs, wikiAbs)
		}
	}

	if _, err := os.Stat(filepath.Join(base, "nested", "escaped.md")); err == nil {
		t.Error("traversal: a file was written outside the wiki directory")
	}
}

func TestAutoIngestRejectsUnusableTopic(t *testing.T) {
	w := NewWikiEngine(t.TempDir())
	if _, err := w.AutoIngest("...", "s", "d"); err == nil {
		t.Error("topic with no usable characters was accepted")
	}
}

// The index is injected into every prompt, so it must be bounded and stable.
func TestGenerateLightweightIndexIsBoundedAndDeterministic(t *testing.T) {
	w := NewWikiEngine(t.TempDir())
	for i := 0; i < maxIndexEntries+10; i++ {
		if _, err := w.AddOrUpdatePage(string(rune('a'+i%26))+"page"+string(rune('0'+i%10))+"x"+itoa(i), "", "", "s", "body", nil); err != nil {
			t.Fatalf("AddOrUpdatePage: %v", err)
		}
	}

	first := w.GenerateLightweightIndex()
	for i := 0; i < 5; i++ {
		if got := w.GenerateLightweightIndex(); got != first {
			t.Fatal("index is not deterministic across calls")
		}
	}

	lines := 0
	for _, r := range first {
		if r == '\n' {
			lines++
		}
	}
	// header + at most maxIndexEntries entries + one "and N more" line
	if lines > maxIndexEntries+1 {
		t.Errorf("index has %d newlines, want at most %d", lines, maxIndexEntries+1)
	}
	if !contains(first, "more pages") {
		t.Error("index did not report the truncated remainder")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func contains(hay, needle string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
