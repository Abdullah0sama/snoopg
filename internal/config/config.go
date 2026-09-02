package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type Profile struct {
	DSN      string `json:"dsn"`
	ReadOnly bool   `json:"read_only,omitempty"`
	TokenCmd string `json:"token_cmd,omitempty"`
}

type Config struct {
	Last     string             `json:"last"`
	Profiles map[string]Profile `json:"profiles"`
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "snoopg", "config.json"), nil
}

func Load() (Config, error) {
	cfg := Config{Profiles: map[string]Profile{}}
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
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg, err
	}
	if last, ok := raw["last"]; ok {
		_ = json.Unmarshal(last, &cfg.Last)
	}
	profiles := raw
	if wrapped, ok := raw["profiles"]; ok {
		_ = json.Unmarshal(wrapped, &profiles)
	}
	for k, v := range profiles {
		if k == "last" {
			continue
		}
		var p Profile
		if err := json.Unmarshal(v, &p); err == nil && p.DSN != "" {
			cfg.Profiles[k] = p
			continue
		}
		var dsn string
		if err := json.Unmarshal(v, &dsn); err == nil && dsn != "" {
			cfg.Profiles[k] = Profile{DSN: dsn}
		}
	}
	return cfg, nil
}

func Save(name, dsn string, readOnly bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Profiles[name] = Profile{DSN: dsn, ReadOnly: readOnly}
	cfg.Last = name
	return write(cfg)
}

func Delete(name string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	delete(cfg.Profiles, name)
	if cfg.Last == name {
		cfg.Last = ""
	}
	return write(cfg)
}

func SetReadOnly(name string, readOnly bool) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	if p, ok := cfg.Profiles[name]; ok {
		p.ReadOnly = readOnly
		cfg.Profiles[name] = p
	}
	return write(cfg)
}

func (c Config) Names() []string {
	out := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func write(cfg Config) error {
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
