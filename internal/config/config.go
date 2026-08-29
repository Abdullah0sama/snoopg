package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Last     string            `json:"last"`
	Profiles map[string]string `json:"profiles"`
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "snoopg", "config.json"), nil
}

func Load() (Config, error) {
	cfg := Config{Profiles: map[string]string{}}
	path, err := Path()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]string{}
	}
	return cfg, nil
}

func Save(name, dsn string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Profiles[name] = dsn
	cfg.Last = name
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
