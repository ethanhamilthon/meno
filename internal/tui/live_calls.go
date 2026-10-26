package tui

import (
	"sort"

	"meno/internal/api"
)

// rebuildLiveCalls derives the live tool calls from the daemon's live state.
//
// A call that has ended is not in the daemon's live state any more, but the
// saved turn that holds its output arrives later (a response with several calls
// is saved once they have all ended). Dropping the call in between makes its
// block vanish for a frame and return, so an ended call stays, with its output,
// until a saved turn carries it. It is drawn as that turn will be, so the
// hand-over changes nothing on screen.
func (rt *Runtime) rebuildLiveCalls(live []api.CallState, started map[string]bool, ended *api.CallEndData) {
	old := make(map[string]liveCall, len(rt.LiveCalls))
	for _, c := range rt.LiveCalls {
		old[c.ID] = c
	}
	var out []liveCall
	for _, c := range rt.LiveCalls {
		if c.Ended && !rt.callSaved(c.ID) {
			out = append(out, c)
		}
	}
	for _, c := range live {
		if c.TaskID != "" {
			continue
		}
		code := c.Command
		if code == "" {
			code = c.Delta
		}
		lc := liveCall{ID: c.CallID, Code: code, Title: c.Title, Running: started[c.CallID], Started: old[c.CallID].Started, Order: old[c.CallID].Order}
		if _, known := old[c.CallID]; !known {
			rt.liveSeq++
			lc.Order = rt.liveSeq
		}
		out = append(out, lc)
	}
	if ended != nil && ended.TaskID == "" {
		if prev, ok := old[ended.CallID]; ok && !prev.Ended {
			prev.Running, prev.Ended = false, true
			prev.Output, prev.ElapsedMs = ended.Output, ended.ElapsedMS
			out = append(out, prev)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	rt.LiveCalls = out
}

// callSaved reports whether a saved turn already holds the call.
func (rt *Runtime) callSaved(id string) bool {
	if id == "" {
		return false
	}
	for _, t := range rt.Session.Turns() {
		if t.CallID == id {
			return true
		}
	}
	return false
}

// dropEndedCalls forgets ended calls that no saved turn will ever replace, for
// example after the run was cut short.
func (rt *Runtime) dropEndedCalls() {
	kept := rt.LiveCalls[:0]
	for _, c := range rt.LiveCalls {
		if !c.Ended {
			kept = append(kept, c)
		}
	}
	rt.LiveCalls = kept
}
