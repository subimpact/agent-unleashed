package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type UserProfile struct {
	PreferredLanguages []string `json:"preferred_languages"`
	Frameworks         []string `json:"frameworks"`
	CodingStyle        []string `json:"coding_style"`
	ActiveProjects     []string `json:"active_projects"`
	DeploymentOS       string   `json:"deployment_os"`
	Notes              []string `json:"notes"`
}

type ProfileManager struct {
	store   *MemoryStore
	profile *UserProfile
	mu      sync.RWMutex
}

func NewProfileManager(store *MemoryStore) *ProfileManager {
	pm := &ProfileManager{
		store: store,
		profile: &UserProfile{
			PreferredLanguages: []string{"Go", "Python"},
			Frameworks:         []string{"FastAPI", "Gin", "modernc.org/sqlite"},
			CodingStyle:        []string{"Clean Go interfaces", "Minimal dependencies", "Single binary"},
			ActiveProjects:     []string{"Agent-Unleashed (agt-ul)"},
			DeploymentOS:       "Windows (PowerShell)",
		},
	}
	pm.loadProfile()
	return pm
}

func (p *ProfileManager) loadProfile() {
	if p.store == nil {
		return
	}
	mems, err := p.store.SearchMemories("user dialectic profile", "user_profile", 1, 0.0)
	if err == nil && len(mems) > 0 {
		var prof UserProfile
		if err := json.Unmarshal([]byte(mems[0].Content), &prof); err == nil {
			p.profile = &prof
		}
	}
}

func (p *ProfileManager) SaveProfile() error {
	p.mu.RLock()
	data, err := json.Marshal(p.profile)
	p.mu.RUnlock()
	if err != nil {
		return err
	}

	if p.store != nil {
		_, err := p.store.AddPalaceMemory("global", "user_profile", "profile", string(data), "dialectic_engine", nil)
		return err
	}
	return nil
}

func (p *ProfileManager) GetSummary() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var lines []string
	lines = append(lines, "[Dialectic User Profile & Coding Persona]:")
	if len(p.profile.PreferredLanguages) > 0 {
		lines = append(lines, fmt.Sprintf("- Preferred Languages: %s", strings.Join(p.profile.PreferredLanguages, ", ")))
	}
	if len(p.profile.CodingStyle) > 0 {
		lines = append(lines, fmt.Sprintf("- Coding Style: %s", strings.Join(p.profile.CodingStyle, " | ")))
	}
	if len(p.profile.ActiveProjects) > 0 {
		lines = append(lines, fmt.Sprintf("- Active Projects: %s", strings.Join(p.profile.ActiveProjects, ", ")))
	}
	return strings.Join(lines, "\n")
}

func (p *ProfileManager) AddPreference(category, item string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch strings.ToLower(category) {
	case "language", "languages":
		p.profile.PreferredLanguages = appendUnique(p.profile.PreferredLanguages, item)
	case "style":
		p.profile.CodingStyle = appendUnique(p.profile.CodingStyle, item)
	case "project":
		p.profile.ActiveProjects = appendUnique(p.profile.ActiveProjects, item)
	default:
		p.profile.Notes = appendUnique(p.profile.Notes, fmt.Sprintf("%s: %s", category, item))
	}
	_ = p.SaveProfile()
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if strings.EqualFold(s, val) {
			return slice
		}
	}
	return append(slice, val)
}
