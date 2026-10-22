package engine

import (
	"context"
	"encoding/json"
	"meno/internal/api"
)

func (e *Engine) attention(id string) (completed, read int64) {
	d, err := e.store.DB()
	if err != nil {
		return
	}
	_ = d.QueryRow("SELECT completed,read_through FROM session_attention WHERE session_id=?", id).Scan(&completed, &read)
	return
}
func (e *Engine) readCompletion(ctx context.Context, id string, raw json.RawMessage) (api.ResourceResponse, error) {
	var q struct {
		Through int64 `json:"through"`
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		return api.ResourceResponse{}, dapi(api.CodeBadRequest, "invalid read acknowledgment")
	}
	s, err := e.load(ctx, id)
	if err != nil {
		return api.ResourceResponse{}, notFound(err)
	}
	s.commit.Lock()
	defer s.commit.Unlock()
	d, err := e.store.DB()
	if err != nil {
		return api.ResourceResponse{}, err
	}
	_, err = d.ExecContext(ctx, `UPDATE session_attention SET read_through=MAX(read_through,MIN(completed,?)) WHERE session_id=?`, q.Through, id)
	if err != nil {
		return api.ResourceResponse{}, err
	}
	e.publishSessionChanged(s, "unread")
	b, _ := json.Marshal(map[string]bool{"ok": true})
	return api.ResourceResponse{Data: b}, nil
}
