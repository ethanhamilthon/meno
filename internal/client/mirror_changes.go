package client

import (
	"encoding/json"
	"meno/internal/proto"
)

func (m *Mirror) applyChange(e proto.Event) ProjectionFields {
	s := &m.State
	if e.Type == proto.EventTasksChanged {
		var d proto.TasksChangedData
		if json.Unmarshal(e.Data, &d) != nil || d.Tasks == nil {
			s.NeedsResync = true
			return 0
		}
		s.Tasks = cloneTasks(d.Tasks)
		return TasksChanged
	}
	var d proto.ChangedData
	if json.Unmarshal(e.Data, &d) != nil {
		s.NeedsResync = true
		return 0
	}
	for _, f := range d.Fields {
		if f == "history" {
			s.NeedsResync = true
			return 0
		}
	}
	if d.Metadata == nil {
		s.NeedsResync = true
		return 0
	}
	v := d.Metadata
	s.Session = v.Session
	s.Cwd = v.Session.Cwd
	s.Model = v.Session.Model
	s.Provider = v.Provider
	s.Effort = v.Effort
	return MetadataChanged
}
