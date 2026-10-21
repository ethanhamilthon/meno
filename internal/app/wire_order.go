package app

import "meno/internal/proto"

// wireOrder guards both watch snapshots and slower metadata refreshes. Only a
// full snapshot advances the cursor: a metadata refresh must not swallow deltas.
type wireOrder struct {
	version proto.SessionVersion
	seq     uint64
	retired map[string]bool
}

func (o *wireOrder) snapshot(s proto.Snapshot, advance bool) bool {
	v := s.Version
	if v.Revision == 0 {
		v.Revision = s.Seq
	}
	if !advance && o.version.BootID != "" && v.BootID != o.version.BootID {
		return false
	}
	if o.retired[v.BootID] || o.version.Newer(v) {
		return false
	}
	if advance {
		if o.version.BootID != "" && v.BootID != o.version.BootID {
			if o.retired == nil {
				o.retired = make(map[string]bool)
			}
			o.retired[o.version.BootID] = true
		}
		o.version, o.seq = v, s.Seq
	}
	return true
}

func (o *wireOrder) event(e proto.Event) bool {
	if e.Type == proto.EventResync {
		return false
	}
	if e.BootID != "" && o.version.BootID != "" && e.BootID != o.version.BootID {
		return false
	}
	if e.Seq == 0 {
		return true
	} // legacy/unsequenced test and local events
	if e.Seq <= o.seq || o.seq > 0 && e.Seq != o.seq+1 {
		return false
	}
	o.seq, o.version.Revision = e.Seq, e.Seq
	if e.BootID != "" {
		o.version.BootID = e.BootID
	}
	return true
}

func staleRuntimeRun(rt *Runtime, rid string) bool {
	return rid != "" && rt.RequestID != "" && rt.Running && rid != rt.RequestID
}
