package daemon

import (
	"context"

	"meno/internal/proto"
)

// Backend owns session and daemon state; the transport layer only routes calls.
type Backend interface {
	ListSessions(context.Context, proto.SessionsListRequest) (proto.SessionsListResponse, error)
	CreateSession(context.Context, proto.SessionCreateRequest) (proto.SessionCreateResponse, error)
	Snapshot(context.Context, string) (proto.Snapshot, error)
	PatchSession(context.Context, string, proto.PatchSessionRequest) (proto.SessionItem, error)
	DeleteSession(context.Context, string) error
	Fork(context.Context, string, proto.ForkRequest) (proto.ForkResponse, error)
	Turns(context.Context, string, proto.TurnsPageRequest) (proto.TurnsPage, error)
	Message(context.Context, string, proto.MessageRequest) (proto.MessageResponse, error)
	Stop(context.Context, string, proto.StopRequest) error
	Compact(context.Context, string, proto.CompactRequest) (proto.CompactResponse, error)
	AnswerAsk(context.Context, string, string, proto.AskAnswerRequest) error
	Resource(context.Context, string, string, string, proto.ResourceRequest) (proto.ResourceResponse, error)
	HasWork(context.Context) bool
	ApplyClientID(context.Context, string, func() (any, error)) (any, error)
}

// NotImplementedBackend is the safe initial daemon backend; it never stores sessions.
type NotImplementedBackend struct{}

func (NotImplementedBackend) err() error {
	return &APIError{Code: proto.CodeNotImplemented, Message: "daemon session backend is not implemented"}
}
func (b NotImplementedBackend) ListSessions(context.Context, proto.SessionsListRequest) (proto.SessionsListResponse, error) {
	return proto.SessionsListResponse{}, b.err()
}
func (b NotImplementedBackend) CreateSession(context.Context, proto.SessionCreateRequest) (proto.SessionCreateResponse, error) {
	return proto.SessionCreateResponse{}, b.err()
}
func (b NotImplementedBackend) Snapshot(context.Context, string) (proto.Snapshot, error) {
	return proto.Snapshot{}, b.err()
}
func (b NotImplementedBackend) PatchSession(context.Context, string, proto.PatchSessionRequest) (proto.SessionItem, error) {
	return proto.SessionItem{}, b.err()
}
func (b NotImplementedBackend) DeleteSession(context.Context, string) error { return b.err() }
func (b NotImplementedBackend) Fork(context.Context, string, proto.ForkRequest) (proto.ForkResponse, error) {
	return proto.ForkResponse{}, b.err()
}
func (b NotImplementedBackend) Turns(context.Context, string, proto.TurnsPageRequest) (proto.TurnsPage, error) {
	return proto.TurnsPage{}, b.err()
}
func (b NotImplementedBackend) Message(context.Context, string, proto.MessageRequest) (proto.MessageResponse, error) {
	return proto.MessageResponse{}, b.err()
}
func (b NotImplementedBackend) Stop(context.Context, string, proto.StopRequest) error { return b.err() }
func (b NotImplementedBackend) Compact(context.Context, string, proto.CompactRequest) (proto.CompactResponse, error) {
	return proto.CompactResponse{}, b.err()
}
func (b NotImplementedBackend) AnswerAsk(context.Context, string, string, proto.AskAnswerRequest) error {
	return b.err()
}
func (b NotImplementedBackend) Resource(context.Context, string, string, string, proto.ResourceRequest) (proto.ResourceResponse, error) {
	return proto.ResourceResponse{}, b.err()
}
func (NotImplementedBackend) HasWork(context.Context) bool { return false }
func (NotImplementedBackend) ApplyClientID(_ context.Context, _ string, fn func() (any, error)) (any, error) {
	return fn()
}
