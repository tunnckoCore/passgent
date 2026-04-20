package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type StoreConfig struct {
	Location  string `toml:"location" json:"location"`
	Identity  string `toml:"identity,omitempty" json:"identity,omitempty"`
	UseGit    bool   `toml:"use_git" json:"use_git"`
	CreatedAt string `toml:"created_at" json:"created_at"`
	UpdatedAt string `toml:"updated_at" json:"updated_at"`
	Commit    string `toml:"commit,omitempty" json:"commit,omitempty"`
}

type Config struct {
	DefaultIdentity     string                 `toml:"default_identity" json:"default_identity"`
	UseEditor           bool                   `toml:"use_editor" json:"use_editor"`
	ClipboardTimeout    int                    `toml:"clipboard_timeout" json:"clipboard_timeout"`
	DisableAutoTraverse bool                   `toml:"disable_auto_traverse" json:"disable_auto_traverse"`
	Presets             map[string]*Preset     `toml:"presets" json:"presets"`
	Stores              map[string]StoreConfig `toml:"stores" json:"stores"`
}

type Preset struct {
	Length        int    `toml:"length,omitempty" json:"length,omitempty"`
	Upper         *bool  `toml:"upper,omitempty" json:"upper,omitempty"`
	Lower         *bool  `toml:"lower,omitempty" json:"lower,omitempty"`
	Numbers       *bool  `toml:"numbers,omitempty" json:"numbers,omitempty"`
	Symbols       *bool  `toml:"symbols,omitempty" json:"symbols,omitempty"`
	Words         int    `toml:"words,omitempty" json:"words,omitempty"`
	Separator     string `toml:"separator,omitempty" json:"separator,omitempty"`
	Count         int    `toml:"count,omitempty" json:"count,omitempty"`
	Pattern       string `toml:"pattern,omitempty" json:"pattern,omitempty"`
	Pronounceable bool   `toml:"pronounceable,omitempty" json:"pronounceable,omitempty"`
	Mnemonic      int    `toml:"mnemonic,omitempty" json:"mnemonic,omitempty"`
	Phrase        int    `toml:"phrase,omitempty" json:"phrase,omitempty"`
	Wordlist      string `toml:"wordlist,omitempty" json:"wordlist,omitempty"`
	Charset       string `toml:"charset,omitempty" json:"charset,omitempty"`
	UUID          string `toml:"uuid,omitempty" json:"uuid,omitempty"`
}

func GetConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "passgent")
}

func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.toml")
}

func GetIdentitiesDir() string {
	return filepath.Join(GetConfigDir(), "identities")
}

func GetDefaultGlobalStoreDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "passgent", "store")
}

func LoadConfig() (*Config, error) {
	cfgPath := GetConfigPath()
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Presets: make(map[string]*Preset),
		Stores:  make(map[string]StoreConfig),
	}

	err = toml.Unmarshal(b, cfg)
	if err != nil {
		return nil, err
	}

	// Defaults for runtime (not saved automatically anymore)
	if cfg.DefaultIdentity == "" {
		cfg.DefaultIdentity = "main"
	}

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	// Collapse paths to use ~ before saving
	for name, s := range cfg.Stores {
		s.Location = CollapseHome(s.Location)
		cfg.Stores[name] = s
	}

	b, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	cfgPath := GetConfigPath()
	os.MkdirAll(filepath.Dir(cfgPath), 0700)
	return os.WriteFile(cfgPath, b, 0600)
}

func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func CollapseHome(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
