package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
	"meno/internal/store"
)

// ThemeColors is the single editable palette. Derived surfaces belong to render.
type ThemeColors struct {
	Dark   bool   `json:"dark"`
	Bg     string `json:"bg"`
	Fg     string `json:"fg"`
	Dim    string `json:"dim"`
	Blue   string `json:"blue"`
	Purple string `json:"purple"`
	Green  string `json:"green"`
	Yellow string `json:"yellow"`
	Red    string `json:"red"`
	Orange string `json:"orange"`
}

func DefaultThemeColors() ThemeColors {
	return ThemeColors{true, "17191C", "DBDFE5", "888F99", "8CB8FF", "B7A5DE", "9DCC9D", "E1CD8C", "E39A9A", "E9BC86"}
}

func (t ThemeColors) Colors() [9]string {
	colors := [9]string{t.Bg, t.Fg, t.Dim, t.Blue, t.Purple, t.Green, t.Yellow, t.Red, t.Orange}
	for i := range colors {
		colors[i] = strings.TrimPrefix(colors[i], "#")
	}
	return colors
}

// ParseTheme validates the complete object before any color can be applied.
func ParseTheme(data []byte) (ThemeColors, error) {
	var t ThemeColors
	if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
		return t, fmt.Errorf("theme.json: expected one palette object")
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return t, fmt.Errorf("theme.json: %w", err)
	}
	for i, c := range t.Colors() {
		c = strings.TrimPrefix(c, "#")
		if len(c) != 6 || strings.IndexFunc(c, func(r rune) bool { return !strings.ContainsRune("0123456789abcdefABCDEF", r) }) >= 0 {
			return t, fmt.Errorf("theme.json: %s must be RRGGBB", []string{"bg", "fg", "dim", "blue", "purple", "green", "yellow", "red", "orange"}[i])
		}
	}
	return t, nil
}

// ReadTheme is read-only, including during partial editor writes and deletion.
func ReadTheme(path string) (ThemeColors, error) {
	f, err := os.Open(path)
	if err != nil {
		return ThemeColors{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil {
		return ThemeColors{}, err
	}
	if len(data) > 65536 {
		return ThemeColors{}, fmt.Errorf("theme.json: palette exceeds 64 KiB")
	}
	return ParseTheme(data)
}

// LoadTheme initializes or upgrades a palette at startup. A dedicated advisory
// lock serializes cooperating windows; rereading before replacement protects edits.
func LoadTheme(path string) (ThemeColors, error) {
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return ThemeColors{}, err
	}
	defer lock.Close()
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return ThemeColors{}, err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN) //nolint:errcheck
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t := DefaultThemeColors()
		raw, _ := json.MarshalIndent(t, "", "  ")
		f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if os.IsExist(e) {
			return ReadTheme(path)
		}
		if e != nil {
			return t, e
		}
		_, e = f.Write(append(raw, '\n'))
		ce := f.Close()
		if e == nil {
			e = ce
		}
		return t, e
	}
	if err != nil {
		return ThemeColors{}, err
	}
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("[")) {
		return ParseTheme(data)
	}
	var legacy []json.RawMessage
	if err = json.Unmarshal(data, &legacy); err != nil {
		return ThemeColors{}, fmt.Errorf("theme.json: %w", err)
	}
	next := DefaultThemeColors()
	for _, raw := range legacy {
		var name struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(raw, &name)
		if name.Name == "panel-dark" {
			if t, e := ParseTheme(raw); e == nil {
				next = t
			} else {
				return next, e
			}
		}
	}
	backup := path + ".legacy.bak"
	f, err := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		old, e := os.ReadFile(backup)
		if e != nil || !bytes.Equal(old, data) {
			return next, fmt.Errorf("theme.json: existing legacy backup differs; original left unchanged")
		}
	} else if err != nil {
		return next, err
	} else {
		_, err = f.Write(data)
		if err == nil {
			err = f.Sync()
		}
		ce := f.Close()
		if err == nil {
			err = ce
		}
		if err != nil {
			return next, err
		}
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return next, err
	}
	if !bytes.Equal(current, data) {
		return next, fmt.Errorf("theme.json: changed during migration; retry startup")
	}
	if err = store.SaveJSON(path, next); err != nil {
		return next, err
	}
	return next, nil
}
