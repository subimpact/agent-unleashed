package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"agent-unleashed/pkg/adapters"
	"agent-unleashed/pkg/config"
	"agent-unleashed/pkg/lcm"
	"agent-unleashed/pkg/memory"
	"agent-unleashed/pkg/skills"
	"agent-unleashed/pkg/wiki"
)

type EventType string

const (
	EventThought    EventType = "thought"
	EventMemory     EventType = "memory_recall"
	EventToolStart  EventType = "tool_start"
	EventToolResult EventType = "tool_result"
	EventText       EventType = "text"
	EventInsight    EventType = "reflection_insight"
	EventUsage      EventType = "usage_stats"
)

type Event struct {
	Type     EventType                 `json:"type"`
	Content  string                    `json:"content,omitempty"`
	Count    int                       `json:"count,omitempty"`
	Memories []string                  `json:"memories,omitempty"`
	ToolName string                    `json:"tool_name,omitempty"`
	Args     interface{}               `json:"args,omitempty"`
	Result   interface{}               `json:"result,omitempty"`
	Insights []string                  `json:"insights,omitempty"`
	Stats    *adapters.ExecutionResult `json:"stats,omitempty"`
}

type UnleashedEngine struct {
	cfg          *config.AppConfig
	MemoryStore  *memory.MemoryStore
	ProfileMgr   *memory.ProfileManager
	SkillIndexer *skills.SkillIndexer
	WikiEngine   *wiki.WikiEngine
	LCMEngine    *lcm.LCMEngine
	Tools        *ToolRunner
	Reflection   *ReflectionEngine
	Registry     *adapters.AdapterRegistry
	activeDriver string
	verbose      bool
	lastResult   *adapters.ExecutionResult
	totalTokens  int
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

	profileMgr := memory.NewProfileManager(store)
	skillIndexer := skills.NewSkillIndexer(cfg.System.SkillsDir)
	wikiEngine := wiki.NewWikiEngine(cfg.Wiki.WikiDir)
	var lcmEngine *lcm.LCMEngine
	if cfg.LCM.Enabled {
		lcmEngine, _ = lcm.NewLCMEngine(cfg.LCM.DBPath)
	}
	toolRunner := NewToolRunner(cfg.System.WorkspaceDir, store)
	refl := NewReflectionEngine(cfg.System.WorkspaceDir, cfg.System.SkillsDir, store)

	reg := adapters.NewAdapterRegistry()
	reg.Register(adapters.NewAgyAdapter(cfg.Model.AgyBinaryPath))
	reg.Register(adapters.NewClaudeAdapter(cfg.Model.ClaudeBinaryPath))
	reg.Register(adapters.NewCodexAdapter(cfg.Model.CodexBinaryPath))
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
		ProfileMgr:   profileMgr,
		SkillIndexer: skillIndexer,
		WikiEngine:   wikiEngine,
		LCMEngine:    lcmEngine,
		Tools:        toolRunner,
		Reflection:   refl,
		Registry:     reg,
		activeDriver: driver,
		verbose:      false,
	}, nil
}

// wikiTopicStopWords are the words a request most often opens with. Keying a
// wiki page on them produced pages called "the", "can" and "please" that were
// then advertised in every subsequent prompt.
var wikiTopicStopWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"can": true, "could": true, "would": true, "should": true, "will": true,
	"do": true, "does": true, "did": true, "is": true, "are": true, "was": true,
	"i": true, "you": true, "we": true, "it": true, "this": true, "that": true,
	"my": true, "your": true, "our": true, "me": true, "us": true,
	"please": true, "hey": true, "hi": true, "hello": true, "ok": true, "okay": true,
	"what": true, "why": true, "how": true, "when": true, "where": true, "who": true,
	"let": true, "lets": true, "just": true, "now": true, "then": true, "also": true,
	"add": true, "fix": true, "make": true, "run": true, "show": true, "get": true,
	"set": true, "use": true, "help": true, "tell": true, "give": true, "need": true,
	"to": true, "for": true, "with": true, "from": true, "on": true, "in": true, "of": true,
}

// wikiTopic picks the first two content-bearing words of a request. It returns
// "" when the message has nothing worth filing, so short or purely
// conversational turns no longer create a page.
func wikiTopic(userMessage string) string {
	if len(strings.TrimSpace(userMessage)) < 15 {
		return ""
	}

	var picked []string
	for _, w := range strings.Fields(strings.ToLower(userMessage)) {
		w = strings.Trim(w, ".,:;!?'\"`()[]{}")
		if len(w) < 3 || wikiTopicStopWords[w] {
			continue
		}
		picked = append(picked, w)
		if len(picked) == 2 {
			break
		}
	}
	if len(picked) == 0 {
		return ""
	}
	return strings.Join(picked, "_")
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

func (e *UnleashedEngine) SetVerbose(v bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.verbose = v
}

func (e *UnleashedEngine) IsVerbose() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.verbose
}

func (e *UnleashedEngine) GetLastResult() *adapters.ExecutionResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastResult
}

func (e *UnleashedEngine) GetTotalTokens() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.totalTokens
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

	var contextBlocks []string

	// 0. Lossless Context Management: replay prior history. This is read before
	// the current turn is appended, so the message being answered is not handed
	// back to the model as its own context.
	if e.LCMEngine != nil && e.cfg.LCM.Enabled {
		if replay, err := e.LCMEngine.BuildContext(sessionID, e.cfg.LCM.ContextTokenBudget); err == nil && replay != "" {
			contextBlocks = append(contextBlocks, replay)
		}
		_, _ = e.LCMEngine.AppendMessage(sessionID, "user", userMessage)
	}

	// 1. Ingest Dialectic User Persona
	if e.ProfileMgr != nil {
		profSummary := e.ProfileMgr.GetSummary()
		if profSummary != "" {
			contextBlocks = append(contextBlocks, profSummary)
		}
	}

	// 2. Progressive Skills Index Injection (Lightweight Narrow Waist)
	if e.SkillIndexer != nil {
		skillIdx := e.SkillIndexer.GenerateLightweightIndex()
		if skillIdx != "" {
			contextBlocks = append(contextBlocks, skillIdx)
		}
	}

	// 3. Query Palace-Mnemosyne Memory
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
				lines = append(lines, fmt.Sprintf("- [%s:%s] %s (relevance: %.2f, decay: %.2f)", m.Room, m.Hall, m.Content, m.Similarity, m.DecayFactor))
			}
			out <- Event{
				Type:     EventMemory,
				Count:    len(memories),
				Memories: memStrings,
			}
			contextBlocks = append(contextBlocks, fmt.Sprintf("[Palace-Mnemosyne Memory Context]:\n%s", strings.Join(lines, "\n")))
		}
	}

	// 4. Inject LLM-Wiki Knowledge Base
	if e.WikiEngine != nil && e.cfg.Wiki.Enabled {
		wikiIdx := e.WikiEngine.GenerateLightweightIndex()
		if wikiIdx != "" {
			contextBlocks = append(contextBlocks, wikiIdx)
		}
	}

	var augmentedPrompt string
	if len(contextBlocks) > 0 {
		augmentedPrompt = strings.Join(contextBlocks, "\n\n") + "\n\n" + userMessage
	} else {
		augmentedPrompt = userMessage
	}

	// 5. Select & Run CLI Adapter
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
		// model.auto_approve_tools was previously declared but never read: every
		// adapter hardcoded its permission bypass, so there was no way to turn it
		// off for untrusted channels.
		"auto_approve": strconv.FormatBool(e.cfg.Model.AutoApproveTools),
	}

	execResult, err := adapter.Execute(ctx, augmentedPrompt, sessionID, e.cfg.System.WorkspaceDir, options)

	e.mu.Lock()
	e.lastResult = execResult
	if execResult != nil {
		e.totalTokens += execResult.TotalTokens
	}
	e.mu.Unlock()

	if err != nil {
		out <- Event{
			Type:    EventText,
			Content: fmt.Sprintf("⚠️ Driver execution error: %v", err),
			Stats:   execResult,
		}
		return
	}

	out <- Event{
		Type:    EventText,
		Content: execResult.Response,
		Stats:   execResult,
	}

	// 6. LLM-Wiki Auto-Ingestion
	if e.WikiEngine != nil && e.cfg.Wiki.Enabled && e.cfg.Wiki.AutoBuild && execResult != nil && execResult.Response != "" {
		if topic := wikiTopic(userMessage); topic != "" {
			_, _ = e.WikiEngine.AutoIngest(topic, userMessage, execResult.Response)
		}
	}

	// 7. Lossless Context Management: Append assistant turn & auto-compress
	if e.LCMEngine != nil && e.cfg.LCM.Enabled && execResult != nil && execResult.Response != "" {
		_, _ = e.LCMEngine.AppendMessage(sessionID, "assistant", execResult.Response)
		if e.cfg.LCM.AutoCompress {
			_, _ = e.LCMEngine.CompressIfExceeds(sessionID, e.cfg.LCM.TokenThreshold)
		}
	}

	out <- Event{
		Type:  EventUsage,
		Stats: execResult,
	}

	// 5. Post-Task Reflection & Self-Evolution
	if e.cfg.Reflection.Enabled && e.Reflection != nil {
		insights := e.Reflection.Reflect(sessionID, userMessage, execResult.Response, nil)
		if len(insights) > 0 {
			out <- Event{
				Type:     EventInsight,
				Insights: insights,
			}
		}
	}
}
