package acp

import (
	"context"

	"meno/internal/api"
	"meno/internal/client"
)

// Backend is the part of the daemon client the adapter uses. *client.Client
// implements it; tests substitute a fake.
type Backend interface {
	CreateSession(ctx context.Context, q api.SessionCreateRequest) (api.SessionCreateResponse, error)
	ListSessions(ctx context.Context, q api.SessionsListRequest) (api.SessionsListResponse, error)
	Snapshot(ctx context.Context, id string) (api.Snapshot, error)
	Turns(ctx context.Context, id string, q api.TurnsPageRequest) (api.TurnsPage, error)
	Message(ctx context.Context, id string, q api.MessageRequest) (api.MessageResponse, error)
	Stop(ctx context.Context, id string, q api.StopRequest) error
	Compact(ctx context.Context, id string, q api.CompactRequest) (api.CompactResponse, error)
	AnswerAsk(ctx context.Context, id, askID string, q api.AskAnswerRequest) error
	PatchSession(ctx context.Context, id string, q api.PatchSessionRequest) (api.SessionItem, error)
	Models(ctx context.Context, q api.ModelsRequest) (api.ModelsResponse, error)
	Skills(ctx context.Context, cwd string) (api.SkillsResponse, error)
	Follow(ctx context.Context, id string, receive func(client.ProjectionUpdate) error) error
}

var _ Backend = (*client.Client)(nil)
