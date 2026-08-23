package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type WikiPage struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Category    string    `json:"category"`
	Summary     string    `json:"summary"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	LastUpdated time.Time `json:"last_updated"`
	Path        string    `json:"path"`
}

type WikiEngine struct {
	wikiDir string
	pages   map[string]*WikiPage
	mu      sync.RWMutex
}

func NewWikiEngine(wikiDir string) *WikiEngine {
	if wikiDir == "" {
		wikiDir = "./.agents/wiki"
	}
	we := &WikiEngine{
		wikiDir: wikiDir,
		pages:   make(map[string]*WikiPage),
	}
	_ = we.Init()
	return we
}

func (w *WikiEngine) Init() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := os.MkdirAll(w.wikiDir, 0755); err != nil {
		return err
	}

	w.pages = make(map[string]*WikiPage)
	entries, err := os.ReadDir(w.wikiDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		slug := strings.TrimSuffix(entry.Name(), ".md")
		filePath := filepath.Join(w.wikiDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		page := w.parsePage(slug, filePath, string(data))
		w.pages[slug] = page
	}

	// Auto-create index if missing
	if _, ok := w.pages["index"]; !ok {
		_ = w.writeIndexPage()
	}

	return nil
}

// maxIndexEntries caps how many wiki pages are advertised in the prompt. The
// index is injected on every single turn, so an uncapped list grows without
// bound as auto-ingest creates pages and quietly eats the context window.
const maxIndexEntries = 25

// SanitizeSlug reduces arbitrary text to a single safe path segment. Auto-ingest
// derives its topic from the first word of a user message, which reaches the
// daemon over unauthenticated channels, so "../.." must not survive into
// filepath.Join.
func SanitizeSlug(raw string) string {
	lowered := strings.ToLower(strings.TrimSpace(raw))

	var b strings.Builder
	for _, r := range lowered {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_', r == '-', r == ' ', r == '.', r == '/', r == '\\', r == ':':
			// Separators and every path metacharacter collapse to one underscore.
			b.WriteRune('_')
		}
	}

	slug := b.String()
	for strings.Contains(slug, "__") {
		slug = strings.ReplaceAll(slug, "__", "_")
	}
	slug = strings.Trim(slug, "_")

	if len(slug) > 30 {
		slug = strings.Trim(slug[:30], "_")
	}
	return slug
}

func titleFromSlug(slug string) string {
	words := strings.Fields(strings.ReplaceAll(slug, "_", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func (w *WikiEngine) parsePage(slug, path, content string) *WikiPage {
	page := &WikiPage{
		Slug:        slug,
		Title:       titleFromSlug(slug),
		Category:    "general",
		Summary:     "",
		Content:     content,
		LastUpdated: time.Now(),
		Path:        path,
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") && page.Title == titleFromSlug(slug) {
			page.Title = strings.TrimPrefix(trimmed, "# ")
		} else if strings.HasPrefix(trimmed, "> ") && page.Summary == "" {
			page.Summary = strings.TrimPrefix(trimmed, "> ")
		}
	}
	if page.Summary == "" && len(lines) > 2 {
		for _, l := range lines[1:] {
			t := strings.TrimSpace(l)
			if t != "" && !strings.HasPrefix(t, "#") {
				page.Summary = t
				if len(page.Summary) > 120 {
					page.Summary = page.Summary[:117] + "..."
				}
				break
			}
		}
	}

	return page
}

func (w *WikiEngine) writeIndexPage() error {
	indexPath := filepath.Join(w.wikiDir, "index.md")
	content := `# 📚 Project LLM-Wiki Index

> Central compiled knowledge graph and architectural reference auto-maintained by Agent-Unleashed.

## 📑 Core Articles
- [[overview]]: High-level project objectives and design tenets.
- [[architecture]]: Core runtime and data flow.
- [[memory_palace]]: Cognitive memory drawers and decay retention.
- [[cli_adapters]]: Supported CLI tools and routing.
- [[cron_engine]]: 24/7 background scheduled automations.
`
	if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
		return err
	}
	w.pages["index"] = w.parsePage("index", indexPath, content)
	return nil
}

func (w *WikiEngine) AddOrUpdatePage(slug, title, category, summary, content string, tags []string) (*WikiPage, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	cleanSlug := SanitizeSlug(slug)
	if cleanSlug == "" {
		return nil, fmt.Errorf("wiki slug %q contains no usable characters", slug)
	}

	if title == "" {
		title = titleFromSlug(cleanSlug)
	}

	filePath := filepath.Join(w.wikiDir, cleanSlug+".md")
	var docBuilder strings.Builder

	docBuilder.WriteString(fmt.Sprintf("# %s\n\n", title))
	if summary != "" {
		docBuilder.WriteString(fmt.Sprintf("> %s\n\n", summary))
	}
	if len(tags) > 0 {
		docBuilder.WriteString(fmt.Sprintf("**Tags:** `%s`\n\n", strings.Join(tags, "`, `")))
	}
	docBuilder.WriteString(fmt.Sprintf("**Last Updated:** %s\n\n---\n\n", time.Now().Format("2006-01-02 15:04:05")))
	docBuilder.WriteString(content)
	docBuilder.WriteString("\n")

	finalContent := docBuilder.String()
	if err := os.WriteFile(filePath, []byte(finalContent), 0644); err != nil {
		return nil, err
	}

	page := &WikiPage{
		Slug:        cleanSlug,
		Title:       title,
		Category:    category,
		Summary:     summary,
		Content:     finalContent,
		Tags:        tags,
		LastUpdated: time.Now(),
		Path:        filePath,
	}
	w.pages[cleanSlug] = page
	return page, nil
}

func (w *WikiEngine) AutoIngest(topic, summary, details string) (*WikiPage, error) {
	slug := SanitizeSlug(topic)
	if slug == "" {
		return nil, fmt.Errorf("wiki topic %q contains no usable characters", topic)
	}

	w.mu.RLock()
	existing, exists := w.pages[slug]
	w.mu.RUnlock()

	if exists && existing != nil {
		// Append section
		newContent := existing.Content + fmt.Sprintf("\n\n### 📝 Auto-Ingested Update (%s)\n%s\n", time.Now().Format("2006-01-02 15:04"), details)
		return w.AddOrUpdatePage(slug, existing.Title, existing.Category, summary, newContent, existing.Tags)
	}

	return w.AddOrUpdatePage(slug, topic, "auto_ingested", summary, details, []string{"auto_generated", "llm_wiki"})
}

func (w *WikiEngine) Search(query string) []*WikiPage {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var matches []*WikiPage
	q := strings.ToLower(query)

	for _, page := range w.pages {
		if strings.Contains(strings.ToLower(page.Title), q) ||
			strings.Contains(strings.ToLower(page.Summary), q) ||
			strings.Contains(strings.ToLower(page.Content), q) ||
			strings.Contains(strings.ToLower(page.Slug), q) {
			matches = append(matches, page)
		}
	}
	return matches
}

func (w *WikiEngine) ListPages() []*WikiPage {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var list []*WikiPage
	for _, p := range w.pages {
		list = append(list, p)
	}
	return list
}

func (w *WikiEngine) GetPage(slug string) (*WikiPage, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	p, ok := w.pages[strings.ToLower(slug)]
	if !ok {
		return nil, fmt.Errorf("wiki page '%s' not found", slug)
	}
	return p, nil
}

func (w *WikiEngine) GenerateLightweightIndex() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if len(w.pages) == 0 {
		return ""
	}

	// Sorted, so the injected prefix is byte-identical between turns. Ranging a
	// map reorders it every call, which defeats provider-side prompt caching.
	slugs := make([]string, 0, len(w.pages))
	for slug := range w.pages {
		if slug == "index" {
			continue
		}
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	truncated := 0
	if len(slugs) > maxIndexEntries {
		truncated = len(slugs) - maxIndexEntries
		slugs = slugs[:maxIndexEntries]
	}
	if len(slugs) == 0 {
		return ""
	}

	lines := make([]string, 0, len(slugs)+2)
	lines = append(lines, "[Project LLM-Wiki Knowledge Base]:")
	for _, slug := range slugs {
		page := w.pages[slug]
		lines = append(lines, fmt.Sprintf("- [[%s]]: %s - %s", slug, page.Title, page.Summary))
	}
	if truncated > 0 {
		lines = append(lines, fmt.Sprintf("- ...and %d more pages (ask to search the wiki)", truncated))
	}
	return strings.Join(lines, "\n")
}
