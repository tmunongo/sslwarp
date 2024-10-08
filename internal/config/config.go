package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ApiKey string `yaml:"api_key"`
	Tunnels map[string]TunnelConfig `yaml:"tunnels"`
}

type TunnelConfig struct {
	Name string `yaml:"name"`
	Addr int `yaml:"addr"`
	Proto string `yaml:"proto"`
	Domain string `yaml:"domain"`
}

func Load(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config

	err = yaml.Unmarshal(file, &cfg)
	if err != nil {
		fmt.Printf("File content: %s\n", string(file))
		return nil, err
	}

	return &cfg, nil
}