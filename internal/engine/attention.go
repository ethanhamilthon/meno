package engine

import (
	"context"
	"encoding/json"
	"meno/internal/proto"
)

func (e *Engine) attention(id string) (completed, read int64) {
	d, err := e.store.DB()
	if err != nil {
		return
	}
	_ = d.QueryRow("SELECT completed,read_through FROM session_attention WHERE session_id=?", id).Scan(&completed, &read)
	return
}
func (e *Engine) readCompletion(ctx context.Context, id string, raw json.RawMessage) (proto.ResourceResponse, error) {
	var q struct {
		Through int64 `json:"through"`
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		return proto.ResourceResponse{}, api(proto.CodeBadRequest, "invalid read acknowledgment")
	}
	d, err := e.store.DB()
	if err != nil {
		return proto.ResourceResponse{}, err
	}
	_, err = d.ExecContext(ctx, `UPDATE session_attention SET read_through=MAX(read_through,MIN(completed,?)) WHERE session_id=?`, q.Through, id)
	if err != nil {
		return proto.ResourceResponse{}, err
	}
	e.publish(id, proto.EventSessionChanged, proto.ChangedData{Fields: []string{"unread"}})
	b, _ := json.Marshal(map[string]bool{"ok": true})
	return proto.ResourceResponse{Data: b}, nil
}
