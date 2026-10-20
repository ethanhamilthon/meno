package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Info returns the metadata of a stored result.
func (s ResultStore) Info(session, id string) (Stored, error) {
	dir, err := s.dir(session, id)
	if err != nil {
		return Stored{}, err
	}
	if b, err := os.ReadFile(filepath.Join(dir, "metadata.json")); err == nil {
		var out Stored
		if err = json.Unmarshal(b, &out); err != nil {
			return Stored{}, err
		}
		return out, nil
	}
	st, err := os.Stat(filepath.Join(dir, "result.json"))
	if err != nil {
		return Stored{}, fmt.Errorf("mcp result %q not found", id)
	}
	out := Stored{ID: id, Size: int(st.Size()), Mime: "application/json", Artifacts: []Artifact{}}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "artifact-") {
			if fi, err := e.Info(); err == nil {
				out.Artifacts = append(out.Artifacts, Artifact{Size: int(fi.Size()), Path: filepath.Join(dir, e.Name())})
			}
		}
	}
	return out, nil
}

// DeleteSession removes everything stored for a session.
func (s ResultStore) DeleteSession(session string) error {
	dir, err := s.dir(session, "placeholder")
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Dir(dir))
}
