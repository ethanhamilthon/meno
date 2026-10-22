package engine

import (
	"context"
	"encoding/json"
	"meno/internal/api"
)

func (e *Engine) globalState(ctx context.Context) (api.ResourceResponse, error) {
	rows, err := e.ListSessions(ctx, api.SessionsListRequest{})
	if err != nil {
		return api.ResourceResponse{}, err
	}
	tasks := 0
	for _, s := range rows.Sessions {
		live, er := e.tasks.List(s.ID, false)
		if er != nil {
			return api.ResourceResponse{}, er
		}
		for _, t := range live {
			if !t.Terminal() {
				tasks++
			}
		}
	}
	data, err := json.Marshal(struct {
		Sessions []api.SessionItem `json:"sessions"`
		Tasks    int               `json:"tasks"`
	}{rows.Sessions, tasks})
	return api.ResourceResponse{Data: data}, err
}
