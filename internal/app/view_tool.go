package app

import (
	"errors"

	"meno/internal/agent"
	"meno/internal/attach"
)

// appViewer implements jsvm.Viewer: images the agent looks at go to the queue
// of its session and reach the model before its next request.
type appViewer struct {
	a   *App
	sid string
	q   *agent.Queue
}

func (v appViewer) View(display string, img attach.Image) (int, error) {
	var refuse error
	ok := v.a.onUI(func() {
		v.a.inRT(v.sid, func() {
			if info, known := v.a.modelInfo(); known && info.Vision != nil && !*info.Vision {
				refuse = errors.New("view: model " + modelOf(v.a) + " does not accept images")
			}
		})
	})
	if !ok {
		return 0, errors.New("view: meno is closing")
	}
	if refuse != nil {
		return 0, refuse
	}
	if v.q.Viewed() >= attach.MaxImages {
		return 0, errors.New("view: at most " + itoa(attach.MaxImages) + " images per step")
	}
	if err := (attach.Store{Dir: v.a.attachDir()}).Put(img); err != nil {
		return 0, err
	}
	n := v.q.Viewed() + 1
	v.q.Push(agent.Message{
		Text:        "Image from view(" + quoteArg(display) + "): " + agent.ImageToken(1),
		Attachments: []agent.Attachment{{N: 1, Mime: img.Mime, SHA: img.SHA, W: img.W, H: img.H}},
		Viewed:      true,
	})
	return n, nil
}

func quoteArg(s string) string { return "\"" + s + "\"" }
