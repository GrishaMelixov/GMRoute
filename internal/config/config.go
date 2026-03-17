package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Rule struct {
	Domain string `yaml:"domain"`
	Route  string `yaml:"route"` // "direct" or "upstream"
}

type Config struct {
	Port     int    `yaml:"port"`
	Upstream string `yaml:"upstream"`
	Rules    []Rule `yaml:"rules"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Port == 0 {
		cfg.Port = 1080
	}
	return &cfg, nil
}
