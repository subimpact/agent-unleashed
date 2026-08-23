package lcm

import (
	"path/filepath"
	"strings"
	"testing"
)

func newEngine(t *testing.T) *LCMEngine {
	t.Helper()
	e, err := NewLCMEngine(filepath.Join(t.TempDir(), "lcm.sqlite"))
	if err != nil {
		t.Fatalf("NewLCMEngine: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

// The store used to be write-only: nothing ever read history back into a
// prompt, so LCM contributed no context at all.
func TestBuildContextReplaysHistory(t *testing.T) {
	e := newEngine(t)

	if _, err := e.AppendMessage("s1", "user", "how does the adapter registry work"); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if _, err := e.AppendMessage("s1", "assistant", "it maps driver names onto CLIAdapter values"); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	got, err := e.BuildContext("s1", 2000)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	for _, want := range []string{"Lossless Context Management", "adapter registry", "CLIAdapter"} {
		if !strings.Contains(got, want) {
			t.Errorf("BuildContext output missing %q:\n%s", want, got)
		}
	}

	// A different session must not see it.
	other, err := e.BuildContext("s2", 2000)
	if err != nil {
		t.Fatalf("BuildContext(s2): %v", err)
	}
	if other != "" {
		t.Errorf("session s2 leaked history from s1: %q", other)
	}
}

func TestBuildContextRespectsTokenBudget(t *testing.T) {
	e := newEngine(t)
	for i := 0; i < 200; i++ {
		if _, err := e.AppendMessage("s1", "user", strings.Repeat("word ", 50)); err != nil {
			t.Fatalf("AppendMessage: %v", err)
		}
	}

	small, err := e.BuildContext("s1", 100)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	large, err := e.BuildContext("s1", 5000)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if len(small) >= len(large) {
		t.Errorf("token budget ignored: small=%d bytes, large=%d bytes", len(small), len(large))
	}
	if small == "" {
		t.Error("a tight budget produced no context at all")
	}
}

// Compression used to re-summarise the same oldest ten messages forever,
// growing summary nodes without bound because nothing was marked as covered.
func TestCompressionConvergesAndDoesNotDuplicate(t *testing.T) {
	e := newEngine(t)
	for i := 0; i < 30; i++ {
		if _, err := e.AppendMessage("s1", "user", strings.Repeat("token ", 40)); err != nil {
			t.Fatalf("AppendMessage: %v", err)
		}
	}

	nodes := 0
	for i := 0; i < 25; i++ {
		node, err := e.CompressIfExceeds("s1", 500)
		if err != nil {
			t.Fatalf("CompressIfExceeds: %v", err)
		}
		if node == nil {
			break
		}
		nodes++
	}

	if nodes == 0 {
		t.Fatal("nothing was ever compressed")
	}
	// 30 messages, 10 per node: at most 3 nodes, and it must stop on its own.
	if nodes > 3 {
		t.Errorf("compression produced %d nodes for 30 messages; it is not converging", nodes)
	}
	if pending := e.PendingTokens("s1"); pending != 0 {
		t.Errorf("PendingTokens = %d after full compression, want 0", pending)
	}
}

// Compression must never lose the verbatim record - that is the "lossless" part.
func TestCompressedMessagesRemainExpandable(t *testing.T) {
	e := newEngine(t)
	first, err := e.AppendMessage("s1", "user", "the original verbatim text")
	if err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	for i := 0; i < 20; i++ {
		if _, err := e.AppendMessage("s1", "user", strings.Repeat("filler ", 40)); err != nil {
			t.Fatalf("AppendMessage: %v", err)
		}
	}
	if _, err := e.CompressIfExceeds("s1", 100); err != nil {
		t.Fatalf("CompressIfExceeds: %v", err)
	}

	got, err := e.Expand(first.ID)
	if err != nil {
		t.Fatalf("Expand after compression: %v", err)
	}
	if got.Content != "the original verbatim text" {
		t.Errorf("verbatim content lost: %q", got.Content)
	}

	hits, err := e.Grep("s1", "original verbatim")
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(hits) == 0 {
		t.Error("compressed message is no longer greppable")
	}
}

func TestBuildContextIncludesSummaryNodes(t *testing.T) {
	e := newEngine(t)
	for i := 0; i < 20; i++ {
		if _, err := e.AppendMessage("s1", "user", strings.Repeat("alpha ", 40)); err != nil {
			t.Fatalf("AppendMessage: %v", err)
		}
	}
	if _, err := e.CompressIfExceeds("s1", 100); err != nil {
		t.Fatalf("CompressIfExceeds: %v", err)
	}

	got, err := e.BuildContext("s1", 2000)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if !strings.Contains(got, "Compressed earlier history") {
		t.Errorf("summary nodes not replayed:\n%s", got)
	}
}

func TestBuildContextOnNilEngine(t *testing.T) {
	var e *LCMEngine
	got, err := e.BuildContext("s1", 100)
	if err != nil || got != "" {
		t.Errorf("nil engine BuildContext = (%q, %v), want (\"\", nil)", got, err)
	}
	if n := e.PendingTokens("s1"); n != 0 {
		t.Errorf("nil engine PendingTokens = %d, want 0", n)
	}
}
