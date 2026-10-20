package engine

import (
	"context"
	"encoding/json"
	"meno/internal/proto"
)

func (e *Engine) globalState(ctx context.Context) (proto.ResourceResponse, error) {
	rows, err := e.ListSessions(ctx, proto.SessionsListRequest{})
	if err != nil {
		return proto.ResourceResponse{}, err
	}
	tasks := 0
	for _, s := range rows.Sessions {
		live, er := e.tasks.List(s.ID, false)
		if er != nil {
			return proto.ResourceResponse{}, er
		}
		for _, t := range live {
			if !t.Terminal() {
				tasks++
			}
		}
	}
	data, err := json.Marshal(struct {
		Sessions []proto.SessionItem `json:"sessions"`
		Tasks    int                 `json:"tasks"`
	}{rows.Sessions, tasks})
	return proto.ResourceResponse{Data: data}, err
}
