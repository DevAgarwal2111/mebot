package agents

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Agent struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
}

type Loader struct {
	AgentsDir string
	mu        sync.RWMutex
	agents    map[string]*Agent
	onChange  func()
}

func NewLoader(agentsDir string) *Loader {
	// Create directory if it doesn't exist
	os.MkdirAll(agentsDir, 0755)

	return &Loader{
		AgentsDir: agentsDir,
		agents:    make(map[string]*Agent),
	}
}

func (l *Loader) SetOnChange(cb func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onChange = cb
}

func (l *Loader) triggerChange() {
	if l.onChange != nil {
		go l.onChange()
	}
}

func (l *Loader) LoadAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.agents = make(map[string]*Agent)

	entries, err := os.ReadDir(l.AgentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(l.AgentsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[Agents] error reading %s: %v", path, err)
			continue
		}

		var agent Agent
		if err := json.Unmarshal(data, &agent); err != nil {
			log.Printf("[Agents] error parsing %s: %v", path, err)
			continue
		}

		l.agents[agent.ID] = &agent
	}

	log.Printf("[Agents] Loaded %d custom agents", len(l.agents))
	return nil
}

func (l *Loader) GetAll() []*Agent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var list []*Agent
	for _, a := range l.agents {
		list = append(list, a)
	}
	return list
}

func (l *Loader) Save(agent *Agent) error {
	if agent.ID == "" {
		agent.ID = strings.ToLower(strings.ReplaceAll(agent.Name, " ", "_"))
	}

	path := filepath.Join(l.AgentsDir, agent.ID+".json")

	data, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	l.mu.Lock()
	l.agents[agent.ID] = agent
	l.mu.Unlock()

	l.triggerChange()
	return nil
}

func (l *Loader) Delete(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.agents[id]; !exists {
		return fmt.Errorf("agent not found: %s", id)
	}

	path := filepath.Join(l.AgentsDir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	delete(l.agents, id)
	l.triggerChange()
	return nil
}
