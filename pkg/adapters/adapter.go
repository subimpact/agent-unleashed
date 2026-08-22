package adapters

import (
	"context"
	"os/exec"
	"sync"
)

type CLIAdapter interface {
	Name() string
	DisplayName() string
	Detect() bool
	BinaryPath() string
	Execute(ctx context.Context, prompt string, sessionID string, workspaceDir string, options map[string]string) (string, error)
	Capabilities() []string
}

type AdapterRegistry struct {
	adapters map[string]CLIAdapter
	mu       sync.RWMutex
}

func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{
		adapters: make(map[string]CLIAdapter),
	}
}

func (r *AdapterRegistry) Register(adapter CLIAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Name()] = adapter
}

func (r *AdapterRegistry) Get(name string) (CLIAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[name]
	return a, ok
}

func (r *AdapterRegistry) ListAvailable() []CLIAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var available []CLIAdapter
	for _, a := range r.adapters {
		if a.Detect() {
			available = append(available, a)
		}
	}
	return available
}

func (r *AdapterRegistry) ListAll() []CLIAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var all []CLIAdapter
	for _, a := range r.adapters {
		all = append(all, a)
	}
	return all
}

// Helper to look up binary in PATH
func FindExecutable(names ...string) (string, bool) {
	for _, name := range names {
		if p, err := exec.LookPath(name); err == nil {
			return p, true
		}
	}
	return "", false
}
