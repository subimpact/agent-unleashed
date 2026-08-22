package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type SkillMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Path        string `yaml:"-"`
}

type SkillIndexer struct {
	skillsDir string
	skills    map[string]*SkillMeta
	mu        sync.RWMutex
}

func NewSkillIndexer(skillsDir string) *SkillIndexer {
	if skillsDir == "" {
		skillsDir = "./.agents/skills"
	}
	si := &SkillIndexer{
		skillsDir: skillsDir,
		skills:    make(map[string]*SkillMeta),
	}
	_ = si.Reindex()
	return si
}

func (s *SkillIndexer) Reindex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.skills = make(map[string]*SkillMeta)
	if _, err := os.Stat(s.skillsDir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(s.skillsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(s.skillsDir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			continue
		}

		meta := s.parseFrontmatter(string(data))
		if meta.Name == "" {
			meta.Name = entry.Name()
		}
		meta.Path = skillFile
		s.skills[meta.Name] = meta
	}

	return nil
}

func (s *SkillIndexer) parseFrontmatter(content string) *SkillMeta {
	meta := &SkillMeta{}
	if !strings.HasPrefix(content, "---") {
		return meta
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) >= 3 {
		_ = yaml.Unmarshal([]byte(parts[1]), meta)
	}
	return meta
}

func (s *SkillIndexer) GenerateLightweightIndex() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.skills) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "[Available Specialized Skills Index]:")
	for name, meta := range s.skills {
		lines = append(lines, fmt.Sprintf("- skill:%s - %s", name, meta.Description))
	}
	return strings.Join(lines, "\n")
}

func (s *SkillIndexer) GetFullSkillContent(name string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, ok := s.skills[name]
	if !ok {
		return "", fmt.Errorf("skill '%s' not found", name)
	}

	data, err := os.ReadFile(meta.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *SkillIndexer) ListSkills() []*SkillMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*SkillMeta
	for _, m := range s.skills {
		list = append(list, m)
	}
	return list
}
