package wiki

import (
	"fmt"
	"os"
	"path/filepath"
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

func (w *WikiEngine) parsePage(slug, path, content string) *WikiPage {
	page := &WikiPage{
		Slug:        slug,
		Title:       strings.Title(strings.ReplaceAll(slug, "_", " ")),
		Category:    "general",
		Summary:     "",
		Content:     content,
		LastUpdated: time.Now(),
		Path:        path,
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") && page.Title == strings.Title(strings.ReplaceAll(slug, "_", " ")) {
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

	cleanSlug := strings.ToLower(strings.TrimSpace(slug))
	cleanSlug = strings.ReplaceAll(cleanSlug, " ", "_")
	cleanSlug = strings.ReplaceAll(cleanSlug, "-", "_")

	if title == "" {
		title = strings.Title(strings.ReplaceAll(cleanSlug, "_", " "))
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
	slug := strings.ToLower(topic)
	slug = strings.ReplaceAll(slug, " ", "_")
	if len(slug) > 30 {
		slug = slug[:30]
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

	var lines []string
	lines = append(lines, "[Project LLM-Wiki Knowledge Base]:")
	for slug, page := range w.pages {
		if slug == "index" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- [[%s]]: %s - %s", slug, page.Title, page.Summary))
	}
	return strings.Join(lines, "\n")
}
