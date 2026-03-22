package skills

import (
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

// CronCallback is called when a scheduled skill triggers.
// It receives the skill that fired.
type CronCallback func(skill *Skill)

// Scheduler manages cron-based skill triggers.
type Scheduler struct {
	mu       sync.Mutex
	cron     *cron.Cron
	loader   *Loader
	callback CronCallback
	entries  map[string]cron.EntryID // skill name → cron entry ID
}

// NewScheduler creates a new skill scheduler.
func NewScheduler(loader *Loader, callback CronCallback) *Scheduler {
	return &Scheduler{
		cron:     cron.New(),
		loader:   loader,
		callback: callback,
		entries:  make(map[string]cron.EntryID),
	}
}

// Start loads all skills with cron expressions and starts the scheduler.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	skills := s.loader.GetAll()
	for _, skill := range skills {
		if skill.CronExpr == "" {
			continue
		}
		s.addSkillCron(skill)
	}

	s.cron.Start()
	log.Printf("[Scheduler] Started with %d scheduled skills", len(s.entries))
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Println("[Scheduler] Stopped")
}

// Reload clears all entries and reloads from the current skill set.
func (s *Scheduler) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove all existing entries
	for name, id := range s.entries {
		s.cron.Remove(id)
		delete(s.entries, name)
	}

	// Re-add from current skills
	skills := s.loader.GetAll()
	for _, skill := range skills {
		if skill.CronExpr == "" {
			continue
		}
		s.addSkillCron(skill)
	}

	log.Printf("[Scheduler] Reloaded with %d scheduled skills", len(s.entries))
}

// addSkillCron registers a single skill's cron job (must hold mu).
func (s *Scheduler) addSkillCron(skill *Skill) {
	// Capture skill in closure
	sk := skill
	id, err := s.cron.AddFunc(sk.CronExpr, func() {
		log.Printf("[Scheduler] Triggering skill: %s", sk.Name)
		if s.callback != nil {
			s.callback(sk)
		}
	})
	if err != nil {
		log.Printf("[Scheduler] Invalid cron expression for skill '%s': %v", sk.Name, err)
		return
	}

	s.entries[sk.Name] = id
	log.Printf("[Scheduler] Scheduled: %s → %s", sk.Name, sk.CronExpr)
}
