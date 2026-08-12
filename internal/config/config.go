package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a YAML-deserializable time.Duration (e.g. "5m", "30s").
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

type Config struct {
	Papra    PapraConfig     `yaml:"papra"`
	Accounts []AccountConfig `yaml:"accounts"`
}

type PapraConfig struct {
	Host   string `yaml:"host"`
	APIKey string `yaml:"api_key"`
}

type AccountConfig struct {
	Name           string   `yaml:"name"`
	Host           string   `yaml:"host"`
	Port           int      `yaml:"port"`
	SSL            bool     `yaml:"ssl"`
	Username       string   `yaml:"username"`
	Password       string   `yaml:"password"`
	Email          string   `yaml:"email"`
	Folder         string   `yaml:"folder"`
	OrganizationID string   `yaml:"organization_id"`
	Tags           []string `yaml:"tags"`
	MarkAsRead     *bool    `yaml:"mark_as_read"`
	PollInterval   Duration `yaml:"poll_interval"`
	// Extensions limits imports to these file extensions (e.g. ["pdf", "docx"]); empty means all.
	Extensions []string `yaml:"extensions"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	for i := range c.Accounts {
		acc := &c.Accounts[i]
		if acc.Port == 0 {
			if acc.SSL {
				acc.Port = 993
			} else {
				acc.Port = 143
			}
		}
		if acc.Folder == "" {
			acc.Folder = "INBOX"
		}
		if acc.PollInterval.Duration == 0 {
			acc.PollInterval.Duration = 5 * time.Minute
		}
		if acc.MarkAsRead == nil {
			markAsRead := true
			acc.MarkAsRead = &markAsRead
		}
		if len(acc.Extensions) == 0 {
			acc.Extensions = []string{"pdf"}
		}
		// Default SSL to true when port is 993
		if acc.Port == 993 && !acc.SSL {
			acc.SSL = true
		}
	}
}
