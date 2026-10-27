package daemon

import (
	"context"

	"meno/internal/api"
)

// Backend owns session and daemon state; the transport layer only routes calls.
type Backend interface {
	ListSessions(context.Context, api.SessionsListRequest) (api.SessionsListResponse, error)
	CreateSession(context.Context, api.SessionCreateRequest) (api.SessionCreateResponse, error)
	Snapshot(context.Context, string) (api.Snapshot, error)
	PatchSession(context.Context, string, api.PatchSessionRequest) (api.SessionItem, error)
	DeleteSession(context.Context, string) error
	Fork(context.Context, string, api.ForkRequest) (api.ForkResponse, error)
	Turns(context.Context, string, api.TurnsPageRequest) (api.TurnsPage, error)
	Message(context.Context, string, api.MessageRequest) (api.MessageResponse, error)
	Stop(context.Context, string, api.StopRequest) error
	Compact(context.Context, string, api.CompactRequest) (api.CompactResponse, error)
	AnswerAsk(context.Context, string, string, api.AskAnswerRequest) error
	Resource(context.Context, string, string, string, api.ResourceRequest) (api.ResourceResponse, error)
	ApplyClientID(context.Context, string, func() (any, error)) (any, error)
}

// NotImplementedBackend is the safe initial daemon backend; it never stores sessions.
type NotImplementedBackend struct{}

func (NotImplementedBackend) err() error {
	return &APIError{Code: api.CodeNotImplemented, Message: "daemon session backend is not implemented"}
}
func (b NotImplementedBackend) ListSessions(context.Context, api.SessionsListRequest) (api.SessionsListResponse, error) {
	return api.SessionsListResponse{}, b.err()
}
func (b NotImplementedBackend) CreateSession(context.Context, api.SessionCreateRequest) (api.SessionCreateResponse, error) {
	return api.SessionCreateResponse{}, b.err()
}
func (b NotImplementedBackend) Snapshot(context.Context, string) (api.Snapshot, error) {
	return api.Snapshot{}, b.err()
}
func (b NotImplementedBackend) PatchSession(context.Context, string, api.PatchSessionRequest) (api.SessionItem, error) {
	return api.SessionItem{}, b.err()
}
func (b NotImplementedBackend) DeleteSession(context.Context, string) error { return b.err() }
func (b NotImplementedBackend) Fork(context.Context, string, api.ForkRequest) (api.ForkResponse, error) {
	return api.ForkResponse{}, b.err()
}
func (b NotImplementedBackend) Turns(context.Context, string, api.TurnsPageRequest) (api.TurnsPage, error) {
	return api.TurnsPage{}, b.err()
}
func (b NotImplementedBackend) Message(context.Context, string, api.MessageRequest) (api.MessageResponse, error) {
	return api.MessageResponse{}, b.err()
}
func (b NotImplementedBackend) Stop(context.Context, string, api.StopRequest) error { return b.err() }
func (b NotImplementedBackend) Compact(context.Context, string, api.CompactRequest) (api.CompactResponse, error) {
	return api.CompactResponse{}, b.err()
}
func (b NotImplementedBackend) AnswerAsk(context.Context, string, string, api.AskAnswerRequest) error {
	return b.err()
}
func (b NotImplementedBackend) Resource(context.Context, string, string, string, api.ResourceRequest) (api.ResourceResponse, error) {
	return api.ResourceResponse{}, b.err()
}
func (NotImplementedBackend) ApplyClientID(_ context.Context, _ string, fn func() (any, error)) (any, error) {
	return fn()
}
