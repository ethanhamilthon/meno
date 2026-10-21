package app

import (
	"meno/internal/client"
	"time"
)

func (a *App) projectionEffects(rt *Runtime, e client.ProjectionEffects) {
	if e.Started != nil {
		if e.Started.Origin != "" && e.Started.Origin != a.instance {
			a.noticeFor(rt, "a turn was started from another window", false, false)
		}
		rt.RunStart, rt.RunTurns, rt.RunJS, rt.RunUsage0 = time.Now(), 0, 0, rt.Usage
		rt.Title = ""
	}
	if e.CallStarted != nil {
		for i := range rt.LiveCalls {
			if rt.LiveCalls[i].ID == e.CallStarted.CallID {
				rt.LiveCalls[i].Started = time.Now()
			}
		}
		if e.CallStarted.Title != "" {
			rt.Title = e.CallStarted.Title
		}
	}
	if e.CallEnded != nil {
		rt.RunJS++
	}
	if e.Usage != nil {
		rt.RunTurns++
	}
	if e.Retry != nil && rt.Retry != nil {
		a.noticeFor(rt, "retry "+itoa(e.Retry.Attempt)+"/"+itoa(e.Retry.Max)+" in "+fmtWait(time.Duration(e.Retry.InMS)*time.Millisecond)+" — "+rt.Retry.Reason, false, false)
	}
	if e.Error != nil {
		a.noticeFor(rt, e.Error.Message, true, false)
	}
	if e.Finished {
		a.markUnseen(rt.ID())
		event := "done"
		if e.Aborted {
			event = ""
		}
		a.signalFor(rt, event)
	}
}
