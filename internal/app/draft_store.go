package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"meno/internal/store"
)

// draftVersion is the file format. A newer file is left alone, never rewritten.
const draftVersion = 1

// draftFile is one unsent message, kept so a crash or restart does not lose it.
// Image bytes live in blobs next to it, named by SHA; the file only names them.
type draftFile struct {
	Version  int          `json:"version"`
	Client   string       `json:"client"`
	Session  string       `json:"session"`
	Text     string       `json:"text"`
	Cursor   int          `json:"cursor"`
	Images   []attachment `json:"images,omitempty"`
	Modified int64        `json:"modified,omitempty"`
}

// draftStore keeps drafts under one directory. Each client instance writes its
// own files (named by client and session), so two windows never overwrite each
// other; a corrupt or foreign file is skipped, not deleted.
type draftStore struct {
	dir string
}

func (d draftStore) path(client, session string) string {
	return filepath.Join(d.dir, safeName(client)+"-"+safeName(session)+".json")
}

func (d draftStore) blob(sha string) string { return filepath.Join(d.dir, "blobs", safeName(sha)) }

func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
}

// newInstanceID is a random identity for one running client instance (one
// terminal window). It is not derived from the process ID, which a restart can
// reuse, and it is distinct from the per-request IDs of newClientID.
func newInstanceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Save writes the blobs first and the metadata after, both atomically, so a
// draft file never names an image that is not on disk. An empty draft removes
// the file.
func (d draftStore) Save(f draftFile, data map[string][]byte) error {
	if f.Text == "" && len(f.Images) == 0 {
		return d.Remove(f.Client, f.Session)
	}
	if err := os.MkdirAll(filepath.Join(d.dir, "blobs"), 0o700); err != nil {
		return err
	}
	for _, im := range f.Images {
		b, ok := data[im.SHA]
		if !ok {
			continue
		}
		if _, err := os.Stat(d.blob(im.SHA)); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(d.blob(im.SHA), b, 0o600); err != nil {
				return err
			}
		}
	}
	f.Version = draftVersion
	return store.SaveJSON(d.path(f.Client, f.Session), f)
}

func (d draftStore) Remove(client, session string) error {
	err := os.Remove(d.path(client, session))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Load reads one draft. A missing file is an empty draft.
func (d draftStore) Load(client, session string) (draftFile, error) {
	var f draftFile
	err := store.LoadJSON(d.path(client, session), &f)
	return f, err
}

// Image returns the stored bytes of an attachment.
func (d draftStore) Image(sha string) ([]byte, error) { return os.ReadFile(d.blob(sha)) }

// Recoverable lists the drafts that belong to other client instances, newest
// format only. Unreadable files are skipped.
func (d draftStore) Recoverable(self string) []draftFile {
	names, _ := filepath.Glob(filepath.Join(d.dir, "*.json"))
	var out []draftFile
	for _, n := range names {
		var f draftFile
		if store.LoadJSON(n, &f) != nil || f.Version < 1 || f.Version > draftVersion || f.Client == self {
			continue
		}
		if f.Text == "" && len(f.Images) == 0 {
			continue
		}
		out = append(out, f)
	}
	return out
}
