package skills

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Loader manages loading and indexing skills from the filesystem.
type Loader struct {
	mu         sync.RWMutex
	skills     map[string]*Skill // keyed by skill name
	builtinDir string
	userDir    string
	onChange   func()
}

// SetOnChange sets a callback to be fired when skills are added, updated, or removed.
func (l *Loader) SetOnChange(fn func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onChange = fn
}

// triggerChange fires the onChange callback in a separate goroutine to prevent deadlocks.
func (l *Loader) triggerChange() {
	if l.onChange != nil {
		go l.onChange()
	}
}

// NewLoader creates a loader and ensures the skill directories exist.
func NewLoader(baseDir string) (*Loader, error) {
	builtinDir := filepath.Join(baseDir, "skills", "builtin")
	userDir := filepath.Join(baseDir, "skills", "user")

	// Create directories if they don't exist
	for _, dir := range []string{builtinDir, userDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create skill directory %s: %w", dir, err)
		}
	}

	return &Loader{
		skills:     make(map[string]*Skill),
		builtinDir: builtinDir,
		userDir:    userDir,
	}, nil
}

// LoadAll scans both builtin and user directories for .md skill files.
func (l *Loader) LoadAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Clear existing
	l.skills = make(map[string]*Skill)

	// Load builtin skills
	builtinCount, err := l.loadDir(l.builtinDir, true)
	if err != nil {
		log.Printf("[Skills] Warning loading builtin skills: %v", err)
	}

	// Load user skills
	userCount, err := l.loadDir(l.userDir, false)
	if err != nil {
		log.Printf("[Skills] Warning loading user skills: %v", err)
	}

	log.Printf("[Skills] Loaded %d built-in skills, %d user skills", builtinCount, userCount)
	return nil
}

// loadDir loads all .md files from a directory.
func (l *Loader) loadDir(dir string, isBuiltIn bool) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		skill, err := parseSkillFile(path, isBuiltIn)
		if err != nil {
			log.Printf("[Skills] Failed to parse %s: %v", path, err)
			continue
		}

		l.skills[skill.Name] = skill
		count++
		log.Printf("[Skills] Loaded: %s (%s)", skill.Name, path)
	}

	return count, nil
}

// parseSkillFile reads a .md file and parses YAML frontmatter + body.
func parseSkillFile(path string, isBuiltIn bool) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	skill := &Skill{
		FilePath:  path,
		IsBuiltIn: isBuiltIn,
	}

	// Parse YAML frontmatter (between --- delimiters)
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			// Parse YAML
			if err := yaml.Unmarshal([]byte(parts[0]), skill); err != nil {
				return nil, fmt.Errorf("invalid YAML frontmatter: %w", err)
			}
			skill.Content = strings.TrimSpace(parts[1])
		} else {
			skill.Content = content
		}
	} else {
		skill.Content = content
	}

	// Fallback: use filename as name if not set
	if skill.Name == "" {
		base := filepath.Base(path)
		skill.Name = strings.TrimSuffix(base, ".md")
	}

	return skill, nil
}

// GetAll returns a copy of all loaded skills.
func (l *Loader) GetAll() []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*Skill, 0, len(l.skills))
	for _, s := range l.skills {
		result = append(result, s)
	}
	return result
}

// Get returns a skill by name.
func (l *Loader) Get(name string) (*Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	s, ok := l.skills[name]
	return s, ok
}

// AddSkill writes a new skill file and adds it to the index.
func (l *Loader) AddSkill(name, content string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Sanitize filename
	safeName := sanitizeFilename(name)
	path := filepath.Join(l.userDir, safeName+".md")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	// Parse and add to index
	skill, err := parseSkillFile(path, false)
	if err != nil {
		return fmt.Errorf("failed to parse new skill: %w", err)
	}

	l.skills[skill.Name] = skill
	log.Printf("[Skills] Created: %s (%s)", skill.Name, path)
	l.triggerChange()
	return nil
}

// UpdateSkill updates an existing user skill's content.
func (l *Loader) UpdateSkill(name, content string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	existing, ok := l.skills[name]
	if !ok {
		return fmt.Errorf("skill '%s' not found", name)
	}
	if existing.IsBuiltIn {
		return fmt.Errorf("cannot edit built-in skill '%s'", name)
	}

	if err := os.WriteFile(existing.FilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	// Re-parse
	skill, err := parseSkillFile(existing.FilePath, false)
	if err != nil {
		return fmt.Errorf("failed to parse updated skill: %w", err)
	}

	l.skills[skill.Name] = skill
	log.Printf("[Skills] Updated: %s", name)
	l.triggerChange()
	return nil
}

// RemoveSkill deletes a user skill file and removes it from the index.
func (l *Loader) RemoveSkill(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	skill, ok := l.skills[name]
	if !ok {
		return fmt.Errorf("skill '%s' not found", name)
	}
	if skill.IsBuiltIn {
		return fmt.Errorf("cannot delete built-in skill '%s'", name)
	}

	if err := os.Remove(skill.FilePath); err != nil {
		return fmt.Errorf("failed to delete skill file: %w", err)
	}

	delete(l.skills, name)
	log.Printf("[Skills] Deleted: %s", name)
	l.triggerChange()
	return nil
}

// UserDir returns the user skills directory path.
func (l *Loader) UserDir() string {
	return l.userDir
}

// sanitizeFilename makes a string safe for use as a filename.
func sanitizeFilename(name string) string {
	// Replace spaces and special chars with underscores
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return strings.ToLower(replacer.Replace(name))
}
