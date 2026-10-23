package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5/pgconn"
)

type Profile struct {
	DSN      string `json:"dsn"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type Config struct {
	Last     string             `json:"last,omitempty"`
	Profiles map[string]Profile `json:"profiles"`
	History  []string           `json:"history,omitempty"`
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
	if hist, ok := raw["history"]; ok {
		_ = json.Unmarshal(hist, &cfg.History)
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

func SaveHistory(entries []string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.History = entries
	return write(cfg)
}

// NameFor derives a profile name from a DSN's database name.
func NameFor(dsn string) string {
	name := "default"
	if cfg, err := pgconn.ParseConfig(dsn); err == nil && cfg.Database != "" {
		name = cfg.Database
	}
	return name
}

// UniqueName returns a profile name derived from the DSN that does not
// collide with an existing profile pointing at a different DSN. This avoids
// silently overwriting a saved connection.
func UniqueName(dsn string) string {
	base := NameFor(dsn)
	cfg, err := Load()
	if err != nil {
		return base
	}
	if p, ok := cfg.Profiles[base]; !ok || p.DSN == dsn {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if _, exists := cfg.Profiles[cand]; !exists {
			return cand
		}
	}
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
