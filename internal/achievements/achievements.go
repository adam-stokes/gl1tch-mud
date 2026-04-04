package achievements

import (
	"os"

	"gopkg.in/yaml.v3"
)

// CatalogFile is the on-disk schema for achievements.yaml.
type CatalogFile struct {
	Source       string        `yaml:"source"`
	Version      string        `yaml:"version"`
	Achievements []Achievement `yaml:"achievements"`
}

// Achievement is one achievement definition.
type Achievement struct {
	ID          string  `yaml:"id"`
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Trigger     Trigger `yaml:"trigger"`
	XP          int     `yaml:"xp"`
}

// Trigger defines when an achievement is earned.
type Trigger struct {
	Action string `yaml:"action"`
	Count  int    `yaml:"count"`
}

// Load reads and parses the catalog file at path.
func Load(path string) (*CatalogFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cf CatalogFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, err
	}
	return &cf, nil
}
