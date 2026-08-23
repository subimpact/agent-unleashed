package memory

import "testing"

// With memory.enabled false the engine leaves *MemoryStore nil. Every method
// must degrade instead of panicking - `agt-ul status`, `memory` and `cron` all
// crashed on this, and the deferred Close() then panicked again mid-unwind.
func TestNilStoreDoesNotPanic(t *testing.T) {
	var s *MemoryStore

	if err := s.Close(); err != nil {
		t.Errorf("Close on nil store: %v", err)
	}
	if db := s.GetDB(); db != nil {
		t.Error("GetDB on nil store returned a handle")
	}
	if stats, err := s.GetStats(); err == nil || stats != nil {
		t.Errorf("GetStats on nil store = (%v, %v), want (nil, error)", stats, err)
	}
	if mems, err := s.GetRecentMemories(5); err == nil || mems != nil {
		t.Errorf("GetRecentMemories on nil store = (%v, %v), want (nil, error)", mems, err)
	}
	if mems, err := s.SearchMemories("q", "", 5, 0); err == nil || mems != nil {
		t.Errorf("SearchMemories on nil store = (%v, %v), want (nil, error)", mems, err)
	}
	if id, err := s.AddPalaceMemory("w", "r", "h", "c", "src", nil); err == nil || id != "" {
		t.Errorf("AddPalaceMemory on nil store = (%q, %v), want (\"\", error)", id, err)
	}
	if id, err := s.AddMemory("fact", "c", "src", nil); err == nil || id != "" {
		t.Errorf("AddMemory on nil store = (%q, %v), want (\"\", error)", id, err)
	}
	if v := s.ComputeEmbedding("text"); v != nil {
		t.Error("ComputeEmbedding on nil store returned a vector")
	}
}
