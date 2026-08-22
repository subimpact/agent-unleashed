package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"agent-unleashed/pkg/adapters"
	"agent-unleashed/pkg/config"
	"agent-unleashed/pkg/memory"
)

type EventType string

const (
	EventThought    EventType = "thought"
	EventMemory     EventType = "memory_recall"
	EventToolStart  EventType = "tool_start"
	EventToolResult EventType = "tool_result"
	EventText       EventType = "text"
	EventInsight    EventType = "reflection_insight"
)

type Event struct {
	Type     EventType   `json:"type"`
	Content  string      `json:"content,omitempty"`
	Count    int         `json:"count,omitempty"`
	Memories []string    `json:"memories,omitempty"`
	ToolName string      `json:"tool_name,omitempty"`
	Args     interface{} `json:"args,omitempty"`
	Result   interface{} `json:"result,omitempty"`
	Insights []string    `json:"insights,omitempty"`
}

type UnleashedEngine struct {
	cfg          *config.AppConfig
	MemoryStore  *memory.MemoryStore
	Tools        *ToolRunner
	Reflection   *ReflectionEngine
	Registry     *adapters.AdapterRegistry
	activeDriver string
	mu           sync.RWMutex
}

func NewUnleashedEngine(cfg *config.AppConfig) (*UnleashedEngine, error) {
	var store *memory.MemoryStore
	var err error

	if cfg.Memory.Enabled {
		store, err = memory.NewMemoryStore(cfg.Memory.DBPath, cfg.Memory.VectorDimension, cfg.Memory.DecayHalfLifeDays)
		if err != nil {
			return nil, fmt.Errorf("failed to init memory store: %w", err)
		}
	}

	toolRunner := NewToolRunner(cfg.System.WorkspaceDir, store)
	refl := NewReflectionEngine(cfg.System.WorkspaceDir, cfg.System.SkillsDir, store)

	reg := adapters.NewAdapterRegistry()
	reg.Register(adapters.NewAgyAdapter(cfg.Model.AgyBinaryPath))
	reg.Register(adapters.NewClaudeAdapter(cfg.Model.ClaudeBinaryPath))
	reg.Register(adapters.NewAiderAdapter(cfg.Model.AiderBinaryPath))
	reg.Register(adapters.NewOllamaAdapter(cfg.Model.OllamaEndpoint))
	reg.Register(adapters.NewAPIAdapter(cfg.Model))

	driver := cfg.Model.Driver
	if driver == "" || driver == "auto" {
		driver = "auto"
	}

	return &UnleashedEngine{
		cfg:          cfg,
		MemoryStore:  store,
		Tools:        toolRunner,
		Reflection:   refl,
		Registry:     reg,
		activeDriver: driver,
	}, nil
}

func (e *UnleashedEngine) SetDriver(driverName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.activeDriver = driverName
}

func (e *UnleashedEngine) GetActiveDriverName() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.activeDriver
}

func (e *UnleashedEngine) resolveAdapter() (adapters.CLIAdapter, string) {
	driverName := e.GetActiveDriverName()

	if driverName != "auto" {
		if a, ok := e.Registry.Get(driverName); ok && a.Detect() {
			return a, a.Name()
		}
	}

	// Auto-cascade priority order: agy -> claude -> aider -> ollama -> api
	priority := []string{"agy", "claude", "aider", "ollama", "api"}
	for _, name := range priority {
		if a, ok := e.Registry.Get(name); ok && a.Detect() {
			return a, a.Name()
		}
	}

	// Fallback to agy adapter even if not in standard path
	if a, ok := e.Registry.Get("agy"); ok {
		return a, "agy"
	}

	return nil, "none"
}

func (e *UnleashedEngine) Chat(ctx context.Context, sessionID, userMessage, channelName string, out chan<- Event) {
	defer close(out)

	// 1. Query Palace-Mnemosyne Memory
	var memoryContext string
	if e.MemoryStore != nil && e.cfg.Memory.Enabled {
		memories, err := e.MemoryStore.SearchMemories(
			userMessage,
			"", // Search across all rooms
			e.cfg.Memory.MaxContextMemories,
			e.cfg.Memory.SimilarityThreshold,
		)
		if err == nil && len(memories) > 0 {
			var memStrings []string
			var lines []string
			for _, m := range memories {
				memStrings = append(memStrings, m.Content)
				lines = append(lines, fmt.Sprintf("- [%s:%s] %s (relevance: %.2f)", m.Room, m.Hall, m.Content, m.Similarity))
			}
			out <- Event{
				Type:     EventMemory,
				Count:    len(memories),
				Memories: memStrings,
			}
			memoryContext = fmt.Sprintf("[Palace-Mnemosyne Memory Context]:\n%s\n\n", strings.Join(lines, "\n"))
		}
	}

	augmentedPrompt := memoryContext + userMessage

	// 2. Select & Run CLI Adapter
	adapter, actualName := e.resolveAdapter()
	if adapter == nil {
		out <- Event{
			Type:    EventText,
			Content: "❌ No AI CLI adapter or API keys detected. Run `agt-ul setup` to configure your agent environment.",
		}
		return
	}

	out <- Event{
		Type:    EventThought,
		Content: fmt.Sprintf("Routing task to driver: %s (%s)...", adapter.DisplayName(), actualName),
	}

	options := map[string]string{
		"effort": e.cfg.Model.Effort,
		"model":  e.cfg.Model.ModelName,
	}

	respText, err := adapter.Execute(ctx, augmentedPrompt, sessionID, e.cfg.System.WorkspaceDir, options)
	if err != nil {
		out <- Event{
			Type:    EventText,
			Content: fmt.Sprintf("⚠️ Driver execution error: %v", err),
		}
		return
	}

	out <- Event{Type: EventText, Content: respText}

	// 3. Post-Task Reflection & Self-Evolution
	if e.cfg.Reflection.Enabled && e.Reflection != nil {
		insights := e.Reflection.Reflect(sessionID, userMessage, respText, nil)
		if len(insights) > 0 {
			out <- Event{
				Type:     EventInsight,
				Insights: insights,
			}
		}
	}
}
