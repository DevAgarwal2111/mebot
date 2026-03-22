package skills

// Skill represents a skill loaded from a markdown file.
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Triggers    []string `yaml:"triggers"`
	Tools       []string `yaml:"tools"`
	CronExpr    string   `yaml:"cron"`
	Content     string   `yaml:"-"` // The markdown body (instructions)
	FilePath    string   `yaml:"-"` // Absolute path to the .md file
	IsBuiltIn   bool     `yaml:"-"` // true if from skills/builtin/
}
