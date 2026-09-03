package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
