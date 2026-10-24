package client

import "meno/internal/api"

// ProjectionFields identifies immutable values present in a projection update.
type ProjectionFields uint32

const (
	MetadataChanged ProjectionFields = 1 << iota
	ActivityChanged
	LiveChanged
	TurnsChanged
	AskChanged
	RetryChanged
	UsageChanged
	QueueChanged
	TasksChanged
	ErrorChanged
	AllChanged = MetadataChanged | ActivityChanged | LiveChanged | TurnsChanged | AskChanged | RetryChanged | UsageChanged | QueueChanged | TasksChanged | ErrorChanged
)

// ProjectionEffects contains accepted, typed UI effects, never raw wire events.
type ProjectionEffects struct {
	Started     *api.StatusData
	CallStarted *api.CallStartData
	CallEnded   *api.CallEndData
	Usage       *api.UsageData
	Retry       *api.RetryData
	Error       *api.ErrorData
	Finished    bool
	Counted     bool
	Aborted     bool
}

// ProjectionUpdate transfers ownership of changed values to its receiver. Turns
// contains only newly appended turns unless Reset is true. No value aliases Mirror.
type ProjectionUpdate struct {
	Version api.SessionVersion
	Seq     uint64
	Reset   bool
	Changed ProjectionFields
	Values  MirrorState
	Effects ProjectionEffects
}

func (m *Mirror) projection(fields ProjectionFields, reset bool, turns []api.Turn, effects ProjectionEffects) ProjectionUpdate {
	s := m.State
	v := MirrorState{}
	if fields&MetadataChanged != 0 {
		v.Session, v.Cwd, v.Model, v.Provider, v.Effort = s.Session, s.Cwd, s.Model, s.Provider, s.Effort
	}
	if fields&ActivityChanged != 0 {
		v.Running, v.Phase, v.RequestID = s.Running, s.Phase, s.RequestID
	}
	if fields&LiveChanged != 0 {
		v.Live = cloneLiveState(s.Live)
		v.StartedCalls = cloneStarted(s.StartedCalls)
	}
	if fields&TurnsChanged != 0 {
		v.Turns = cloneTurns(turns)
	}
	if fields&AskChanged != 0 {
		v.Ask = cloneAsk(s.Ask)
	}
	if fields&RetryChanged != 0 {
		v.Retry = cloneRetry(s.Retry)
		v.RetryMax = s.RetryMax
	}
	if fields&UsageChanged != 0 {
		v.Metrics, v.LastUsage, v.TotalUsage, v.Spent, v.Context, v.Unpriced = s.Metrics, s.LastUsage, s.TotalUsage, s.Spent, s.Context, s.Unpriced
	}
	if fields&QueueChanged != 0 {
		v.Queued = append([]string(nil), s.Queued...)
	}
	if fields&TasksChanged != 0 {
		v.Tasks = cloneTasks(s.Tasks)
	}
	if fields&ErrorChanged != 0 {
		v.LastError = s.LastError
	}
	return ProjectionUpdate{Version: s.Version, Seq: s.Seq, Reset: reset, Changed: fields, Values: v, Effects: effects}
}
