package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"meno/internal/llm"
	"meno/internal/store"
)

// decodeProviders splits the entries of provider.json into the valid providers
// and the raw entries that were rejected (broken JSON, failed validation, a
// repeated name). The rejected ones are kept when the file is saved again.
func decodeProviders(raw []json.RawMessage, file string) (ok []llm.Provider, bad []json.RawMessage, warns []string) {
	seen := map[string]bool{}
	for i, r := range raw {
		var p llm.Provider
		if err := json.Unmarshal(r, &p); err != nil {
			warns = append(warns, fmt.Sprintf("%s: entry %d: %v", file, i+1, err))
			bad = append(bad, r)
			continue
		}
		if err := llm.Validate(p); err != nil {
			warns = append(warns, fmt.Sprintf("%s: %s: %v", file, label(p.Name, i), err))
			bad = append(bad, r)
			continue
		}
		if seen[p.Name] {
			warns = append(warns, fmt.Sprintf("%s: %s: duplicate name, the first one is used", file, p.Name))
			bad = append(bad, r)
			continue
		}
		seen[p.Name] = true
		ok = append(ok, p)
	}
	return ok, bad, warns
}

// LoadProvidersFile reads provider.json with its warnings. An api_key of the
// form env:NAME is returned as written. An invalid entry is skipped.
func (s *Store) LoadProvidersFile() ([]llm.Provider, []string, error) {
	listMu.Lock()
	defer listMu.Unlock()
	raw, err := readFile(s.P.ProviderFile())
	if err != nil {
		return nil, nil, err
	}
	ps, _, warns := decodeProviders(raw, ProviderFileName)
	return ps, warns, nil
}

// LoadProviders returns the providers in provider.json as written (env:NAME is
// not replaced). Use LoadProvidersResolved to run requests.
func (s *Store) LoadProviders() ([]llm.Provider, error) {
	ps, _, err := s.LoadProvidersFile()
	return ps, err
}

// LoadProvidersResolved is LoadProviders with every api_key of the form
// env:NAME replaced by the value of that environment variable.
func (s *Store) LoadProvidersResolved(getenv func(string) string) ([]llm.Provider, error) {
	ps, err := s.LoadProviders()
	if err != nil {
		return nil, err
	}
	return ResolveKeys(ps, getenv), nil
}

// ResolveKeys replaces env:NAME keys with the environment value; it copies the
// list and leaves the argument alone.
func ResolveKeys(ps []llm.Provider, getenv func(string) string) []llm.Provider {
	out := append([]llm.Provider(nil), ps...)
	for i := range out {
		if name, ok := strings.CutPrefix(out[i].APIKey, "env:"); ok {
			out[i].APIKey = getenv(strings.TrimSpace(name))
		}
	}
	return out
}

// SaveProviders writes the providers to provider.json. Entries that could not be read
// stay in it as they were; a file that is not a JSON array is not touched.
func (s *Store) SaveProviders(ps []llm.Provider) error {
	listMu.Lock()
	defer listMu.Unlock()
	raw, err := readForWrite(s.P.ProviderFile())
	if err != nil {
		return err
	}
	_, bad, _ := decodeProviders(raw, ProviderFileName)
	return s.saveProviderEntries(ps, bad)
}

// saveProviderEntries writes ps followed by the raw entries that were rejected.
func (s *Store) saveProviderEntries(ps []llm.Provider, bad []json.RawMessage) error {
	out := []json.RawMessage{}
	for _, p := range ps {
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}
		out = append(out, b)
	}
	return store.SaveJSON(s.P.ProviderFile(), append(out, bad...))
}

// Getenv is os.Getenv; callers pass it to the resolving functions.
var Getenv = os.Getenv
