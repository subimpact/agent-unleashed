package adapters

import (
	"testing"
)

func TestAdapterRegistry(t *testing.T) {
	reg := NewAdapterRegistry()

	agy := NewAgyAdapter("")
	claude := NewClaudeAdapter("")
	codex := NewCodexAdapter("")
	aider := NewAiderAdapter("")
	ollama := NewOllamaAdapter("")

	reg.Register(agy)
	reg.Register(claude)
	reg.Register(codex)
	reg.Register(aider)
	reg.Register(ollama)

	all := reg.ListAll()
	if len(all) != 5 {
		t.Fatalf("Expected 5 registered adapters, got %d", len(all))
	}

	a, ok := reg.Get("agy")
	if !ok || a.Name() != "agy" {
		t.Fatalf("Failed to retrieve agy adapter")
	}

	if len(a.Capabilities()) == 0 {
		t.Fatal("Expected capabilities on agy adapter")
	}
}
