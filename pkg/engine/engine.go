package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"antigravity-unleashed/pkg/config"
	"antigravity-unleashed/pkg/memory"
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

type AgyResponse struct {
	ConversationID  string                 `json:"conversation_id"`
	Status          string                 `json:"status"`
	Response        string                 `json:"response"`
	DurationSeconds float64                `json:"duration_seconds"`
	NumTurns        int                    `json:"num_turns"`
	Usage           map[string]interface{} `json:"usage"`
}

type UnleashedEngine struct {
	cfg          *config.AppConfig
	MemoryStore  *memory.MemoryStore
	Tools        *ToolRunner
	Reflection   *ReflectionEngine
	sessions     map[string]string
	sessionsLock sync.RWMutex
}

func NewUnleashedEngine(cfg *config.AppConfig) (*UnleashedEngine, error) {
	var store *memory.MemoryStore
	var err error

	if cfg.Memory.Enabled {
		store, err = memory.NewMemoryStore(cfg.Memory.DBPath, cfg.Memory.VectorDimension)
		if err != nil {
			return nil, fmt.Errorf("failed to init memory store: %w", err)
		}
	}

	toolRunner := NewToolRunner(cfg.System.WorkspaceDir, store)
	refl := NewReflectionEngine(cfg.System.WorkspaceDir, cfg.System.SkillsDir, store)

	return &UnleashedEngine{
		cfg:         cfg,
		MemoryStore: store,
		Tools:       toolRunner,
		Reflection:  refl,
		sessions:    make(map[string]string),
	}, nil
}

func (e *UnleashedEngine) ResolveAgyPath() string {
	if e.cfg.Model.AgyBinaryPath != "" {
		if _, err := os.Stat(e.cfg.Model.AgyBinaryPath); err == nil {
			return e.cfg.Model.AgyBinaryPath
		}
	}

	if p, err := exec.LookPath("agy"); err == nil {
		return p
	}
	if p, err := exec.LookPath("agy.exe"); err == nil {
		return p
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		candidate := filepath.Join(localAppData, "agy", "bin", "agy.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return "agy"
}

func (e *UnleashedEngine) Chat(ctx context.Context, sessionID, userMessage, channelName string, out chan<- Event) {
	defer close(out)

	// 1. Query Persistent Memory
	var memoryContext string
	if e.MemoryStore != nil && e.cfg.Memory.Enabled {
		memories, err := e.MemoryStore.SearchMemories(
			userMessage,
			"",
			e.cfg.Memory.MaxContextMemories,
			e.cfg.Memory.SimilarityThreshold,
		)
		if err == nil && len(memories) > 0 {
			var memStrings []string
			var lines []string
			for _, m := range memories {
				memStrings = append(memStrings, m.Content)
				lines = append(lines, fmt.Sprintf("- [%s] %s", m.Category, m.Content))
			}
			out <- Event{
				Type:     EventMemory,
				Count:    len(memories),
				Memories: memStrings,
			}
			memoryContext = fmt.Sprintf("[Persistent Memory Context]:\n%s\n\n", strings.Join(lines, "\n"))
		}
	}

	augmentedPrompt := memoryContext + userMessage
	var responseText string

	// 2. Dispatch to local Antigravity CLI session (Zero-API-Key)
	agyBin := e.ResolveAgyPath()
	out <- Event{
		Type:    EventThought,
		Content: fmt.Sprintf("Querying local Antigravity CLI (%s)...", agyBin),
	}

	args := []string{
		"--output-format", "json",
		fmt.Sprintf("--effort=%s", e.cfg.Model.Effort),
		fmt.Sprintf("--print=%s", augmentedPrompt),
	}

	if e.cfg.Model.AutoApproveTools {
		args = append(args, "--dangerously-skip-permissions")
	}

	e.sessionsLock.RLock()
	convID, hasConv := e.sessions[sessionID]
	e.sessionsLock.RUnlock()

	if hasConv && convID != "" {
		args = append(args, fmt.Sprintf("--conversation=%s", convID))
	}

	cmd := exec.CommandContext(ctx, agyBin, args...)
	cmd.Dir = e.cfg.System.WorkspaceDir

	rawOut, err := cmd.CombinedOutput()
	if err != nil {
		responseText = fmt.Sprintf("⚠️ Execution error: %v\nOutput: %s", err, string(rawOut))
		out <- Event{Type: EventText, Content: responseText}
		return
	}

	var agyResp AgyResponse
	if err := json.Unmarshal(rawOut, &agyResp); err == nil && agyResp.Response != "" {
		responseText = agyResp.Response
		if agyResp.ConversationID != "" {
			e.sessionsLock.Lock()
			e.sessions[sessionID] = agyResp.ConversationID
			e.sessionsLock.Unlock()
		}
	} else {
		responseText = string(rawOut)
	}

	out <- Event{Type: EventText, Content: responseText}

	// 3. Trigger Post-Task Reflection
	if e.cfg.Reflection.Enabled && e.Reflection != nil {
		insights := e.Reflection.Reflect(sessionID, userMessage, responseText, nil)
		if len(insights) > 0 {
			out <- Event{
				Type:     EventInsight,
				Insights: insights,
			}
		}
	}
}
