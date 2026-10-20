package mcp

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Retention limits: a result is stored whole, up to these sizes, and the
// result says so when something did not fit.
const (
	MaxResultBytes   = 64 << 20
	MaxArtifactBytes = 32 << 20
	DefaultReadBytes = 64 << 10
)

// Artifact is a binary part of a result, materialized as a local file.
type Artifact struct {
	Index int    `json:"index"`
	Kind  string `json:"kind"` // image, audio, resource
	Mime  string `json:"mime"`
	Size  int    `json:"size"`
	Path  string `json:"path"`
	URI   string `json:"uri,omitempty"`
}

// Stored describes a result kept on disk.
type Stored struct {
	ID        string     `json:"id"`
	Server    string     `json:"server"`
	Tool      string     `json:"tool"`
	Size      int        `json:"size"`
	Mime      string     `json:"mime"`
	Artifacts []Artifact `json:"artifacts"`
	// Notes lists anything that was not retained (too large, undecodable).
	Notes []string `json:"notes,omitempty"`
}

// ResultStore keeps tool results under Dir/<session>/<id>/. Results belong to
// the session that made the call; another session cannot read them.
type ResultStore struct{ Dir string }

func (s ResultStore) dir(session, id string) (string, error) {
	if session == "" || id == "" || strings.ContainsAny(session+id, "/\\.") {
		return "", errors.New("invalid result id")
	}
	return filepath.Join(s.Dir, session, id), nil
}

// Save stores the complete result. Binary blocks go to files; the canonical
// JSON keeps everything else, with the base64 payloads replaced by references.
func (s ResultStore) Save(session, server, tool string, res Result) (Stored, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return Stored{}, err
	}
	id := "r" + hex.EncodeToString(b[:])
	dir, err := s.dir(session, id)
	if err != nil {
		return Stored{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Stored{}, err
	}
	st := Stored{ID: id, Server: server, Tool: tool, Mime: "application/json", Artifacts: []Artifact{}}
	canon := map[string]any{}
	if len(res.Raw) > 0 {
		if err := json.Unmarshal(res.Raw, &canon); err != nil {
			return st, err
		}
	}
	if canon == nil {
		return st, errors.New("mcp: result must be an object")
	}
	canon["isError"] = res.IsError
	canon["content"] = []any{}
	if len(res.Content) == 0 && res.Text != "" {
		raw, _ := json.Marshal(map[string]any{"type": "text", "text": res.Text})
		res.Content = []json.RawMessage{raw}
	}
	var blocks []any
	for i, raw := range res.Content {
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			blocks = append(blocks, json.RawMessage(raw))
			continue
		}
		s.extract(dir, i, m, &st)
		blocks = append(blocks, m)
	}
	if blocks != nil {
		canon["content"] = blocks
	}
	if res.Structured != nil {
		canon["structuredContent"] = res.Structured
	}
	data, err := json.MarshalIndent(canon, "", "  ")
	if err != nil {
		return st, err
	}
	if len(data) > MaxResultBytes {
		st.Notes = append(st.Notes, fmt.Sprintf("result is %d bytes; a bounded preview replaces it (limit %d)", len(data), MaxResultBytes))
		data, _ = json.Marshal(map[string]any{"isError": res.IsError, "content": []any{}, "retentionError": "result exceeded retention limit", "preview": cutUTF8(string(data), MaxResultBytes/8)})
	}
	st.Size = len(data)
	if err := os.WriteFile(filepath.Join(dir, "result.json"), data, 0o600); err != nil {
		return st, err
	}
	meta, err := json.Marshal(st)
	if err != nil {
		return st, err
	}
	if err = os.WriteFile(filepath.Join(dir, "metadata.json"), meta, 0o600); err != nil {
		return st, err
	}
	return st, nil
}

// extract moves the base64 payload of an image/audio/resource block to a file.
func (s ResultStore) extract(dir string, idx int, m map[string]any, st *Stored) {
	kind, _ := m["type"].(string)
	holder, data, mime := m, "", ""
	if kind == "resource" {
		if r, ok := m["resource"].(map[string]any); ok {
			holder = r
		}
		data, _ = holder["blob"].(string)
	} else if kind == "image" || kind == "audio" {
		data, _ = m["data"].(string)
	}
	mime, _ = holder["mimeType"].(string)
	if data == "" {
		return
	}
	key := "data"
	if kind == "resource" {
		key = "blob"
	}
	if base64.StdEncoding.DecodedLen(len(data)) > MaxArtifactBytes+2 {
		st.Notes = append(st.Notes, fmt.Sprintf("block %d: encoded payload exceeds artifact limit; not retained", idx))
		delete(holder, key)
		return
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		st.Notes = append(st.Notes, fmt.Sprintf("block %d: payload is not valid base64; not materialized", idx))
		delete(holder, key)
		return
	}
	if len(raw) > MaxArtifactBytes {
		st.Notes = append(st.Notes, fmt.Sprintf("block %d: %d bytes exceeds the artifact limit of %d; not retained", idx, len(raw), MaxArtifactBytes))
		delete(holder, key)
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("artifact-%d%s", idx, extFor(mime)))
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		st.Notes = append(st.Notes, fmt.Sprintf("block %d: cannot store artifact: %v", idx, err))
		delete(holder, key)
		return
	}
	uri, _ := holder["uri"].(string)
	st.Artifacts = append(st.Artifacts, Artifact{Index: idx, Kind: kind, Mime: mime, Size: len(raw), Path: path, URI: uri})
	delete(holder, key)
	holder["artifact"] = path
	holder["size"] = len(raw)
}

func extFor(mime string) string {
	switch strings.ToLower(strings.SplitN(mime, ";", 2)[0]) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "application/pdf":
		return ".pdf"
	}
	return ".bin"
}

// Slice is a bounded part of the canonical representation of a result.
type Slice struct {
	Text       string `json:"text"`
	Offset     int    `json:"offset"`
	End        int    `json:"end"`
	Size       int    `json:"size"`
	NextOffset *int   `json:"nextOffset"`
}

// Read returns up to limit bytes from offset, moved forward to a UTF-8 boundary
// and ending on one.
func (s ResultStore) Read(session, id string, offset, limit int) (Slice, error) {
	dir, err := s.dir(session, id)
	if err != nil {
		return Slice{}, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "result.json"))
	if err != nil {
		return Slice{}, fmt.Errorf("mcp result %q not found", id)
	}
	if limit <= 0 || limit > DefaultReadBytes {
		limit = DefaultReadBytes
	}
	if offset < 0 || offset > len(b) {
		return Slice{}, fmt.Errorf("offset %d is outside the result (%d bytes)", offset, len(b))
	}
	for offset < len(b) && !utf8.RuneStart(b[offset]) {
		offset++
	}
	end := min(offset+limit, len(b))
	for end < len(b) && end > offset && !utf8.RuneStart(b[end]) {
		end--
	}
	if end == offset && offset < len(b) {
		return Slice{}, fmt.Errorf("limit %d is too small for the next UTF-8 character", limit)
	}
	out := Slice{Text: string(b[offset:end]), Offset: offset, End: end, Size: len(b)}
	if end < len(b) {
		out.NextOffset = &end
	}
	return out, nil
}

func cutUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
