package store

import (
	"fmt"
	"os"
	"path/filepath"

	"passgent-go/internal/config"
)

func ResolveStore(cfg *config.Config, name string) (string, error) {
	if name != "" {
		if s, ok := cfg.Stores[name]; ok {
			return config.ExpandHome(s.Location), nil
		}
		return "", fmt.Errorf("store %q not found in config", name)
	}

	if cfg.DisableAutoTraverse {
		return config.ExpandHome(config.GetDefaultGlobalStoreDir()), nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return config.ExpandHome(config.GetDefaultGlobalStoreDir()), nil
	}

	for {
		passgentPath := filepath.Join(dir, ".passgent", "store")
		info, err := os.Stat(passgentPath)
		if err == nil && info.IsDir() {
			return passgentPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir || parent == "/" {
			break
		}
		dir = parent
	}

	return config.ExpandHome(config.GetDefaultGlobalStoreDir()), nil
}
