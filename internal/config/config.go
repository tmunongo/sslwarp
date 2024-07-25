package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ApiKey string `yaml:"api_key"`
	Tunnels map[string]TunnelConfig `yaml:"tunnels"`
}

type TunnelConfig struct {
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
		return nil, err
	}

	return &cfg, nil

}