package config

import (
	"database/sql"
	"errors"
	"os"
	"sync"

	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/store"
)

var (
	importMu   sync.Mutex
	importDone = map[*sql.DB]bool{}
)

// importLegacy copies settings.json, providers.json, mcp.json, skills.json and
// model-catalog.json into the database once. The files stay where they are.
// A failed import changes nothing and is retried on the next start.
func (s *Store) importLegacy(d *sql.DB) error {
	importMu.Lock()
	defer importMu.Unlock()
	if importDone[d] {
		return nil
	}
	var done int
	if err := d.QueryRow("SELECT COUNT(1) FROM meta WHERE key='migrated_json'").Scan(&done); err != nil {
		return err
	}
	if done == 0 {
		err := runTx(d, func(t *sql.Tx) error {
			var st Settings
			if has, err := loadFile(s.P.File("settings.json"), &st); err != nil {
				return err
			} else if has {
				if err := putSettings(t, st); err != nil {
					return err
				}
			}
			var ps []llm.Provider
			if has, err := loadFile(s.P.File("providers.json"), &ps); err != nil {
				return err
			} else if has {
				if err := putProviders(t, ps); err != nil {
					return err
				}
			}
			var sv []mcp.Server
			if has, err := loadFile(s.P.File("mcp.json"), &sv); err != nil {
				return err
			} else if has {
				if isLegacyPlaceholders(sv) {
					sv = nil
				}
				if err := putServers(t, sv); err != nil {
					return err
				}
			}
			var sk skillState
			if has, _ := loadFile(s.P.File("skills.json"), &sk); has {
				for _, n := range sk.Disabled {
					if _, err := t.Exec(`INSERT INTO skills_state(name,enabled) VALUES(?,0)
						ON CONFLICT(name) DO UPDATE SET enabled=0`, n); err != nil {
						return err
					}
				}
			}
			var cat map[string]llm.ModelInfo
			if has, err := loadFile(s.P.File("model-catalog.json"), &cat); err == nil && has {
				if err := putCatalog(t, cat); err != nil {
					return err
				}
			}
			_, err := t.Exec("INSERT INTO meta(key,value) VALUES('migrated_json','1')")
			return err
		})
		if err != nil {
			return err
		}
	}
	importDone[d] = true
	return nil
}

// loadFile decodes a JSON file; has is false when it does not exist or is empty.
func loadFile(path string, v any) (has bool, err error) {
	if _, serr := os.Stat(path); errors.Is(serr, os.ErrNotExist) {
		return false, nil
	}
	if err := store.LoadJSON(path, v); err != nil {
		return false, err
	}
	return true, nil
}
