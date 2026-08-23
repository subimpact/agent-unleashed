package skills

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

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

	// Sorted, so the injected prefix is byte-identical between turns. Ranging a
	// map reorders it every call, which defeats provider-side prompt caching.
	names := make([]string, 0, len(s.skills))
	for name := range s.skills {
		names = append(names, name)
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names)+1)
	lines = append(lines, "[Available Specialized Skills Index]:")
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("- skill:%s - %s", name, s.skills[name].Description))
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

func (s *SkillIndexer) InstallSkill(source string) (*SkillMeta, error) {
	cleanSource := strings.TrimSpace(source)
	cleanSource = strings.TrimPrefix(cleanSource, "https://github.com/")
	cleanSource = strings.TrimSuffix(cleanSource, ".git")

	// 1. Try Direct Raw GitHub Fetch
	var rawURL string
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		if strings.Contains(source, "raw.githubusercontent.com") {
			rawURL = source
		} else {
			rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/main/skills/taste-skill/SKILL.md", cleanSource)
		}
	} else {
		rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/main/SKILL.md", cleanSource)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(rawURL)

	var content string
	if err == nil && resp.StatusCode == 200 {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		content = string(data)
	} else {
		// Fallback: git clone shallow into temp dir
		tempDir, err := os.MkdirTemp("", "skill_install_*")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tempDir)

		gitRepo := source
		if !strings.HasPrefix(source, "http") {
			gitRepo = "https://github.com/" + source + ".git"
		}

		cmd := exec.Command("git", "clone", "--depth=1", gitRepo, tempDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("could not install skill from %s: %v (%s)", source, err, string(out))
		}

		// Find SKILL.md in cloned repo
		var foundPath string
		_ = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
			if info != nil && !info.IsDir() && strings.EqualFold(info.Name(), "SKILL.md") {
				foundPath = path
				return filepath.SkipDir
			}
			return nil
		})

		if foundPath == "" {
			return nil, fmt.Errorf("no SKILL.md found in repository %s", source)
		}

		data, err := os.ReadFile(foundPath)
		if err != nil {
			return nil, err
		}
		content = string(data)
	}

	meta := s.parseFrontmatter(content)
	skillName := meta.Name
	if skillName == "" {
		parts := strings.Split(cleanSource, "/")
		skillName = parts[len(parts)-1]
	}

	targetDir := filepath.Join(s.skillsDir, skillName)
	_ = os.MkdirAll(targetDir, 0755)
	destFile := filepath.Join(targetDir, "SKILL.md")

	if err := os.WriteFile(destFile, []byte(content), 0644); err != nil {
		return nil, err
	}

	meta.Path = destFile
	_ = s.Reindex()
	return meta, nil
}
