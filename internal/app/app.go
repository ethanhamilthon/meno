package app

import (
	"context"
	"sync"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/jsvm"
	"meno/internal/llm"
	"meno/internal/mcp"
)

type App struct {
	scr    tcell.Screen
	st     *State
	store  *config.Store
	paths  config.Paths
	vm     *jsvm.VM
	gate   *jsvm.Gate
	mcp    *mcp.Manager
	client func(llm.Provider) llm.Client
	events chan any
	quit   bool

	mu      sync.Mutex
	cancel  context.CancelFunc
	running sync.WaitGroup
}

type agentEv struct{ ev agent.Ev }
type turnDone struct{ err error }
type approvalReq struct {
	action, target string
	reply          chan bool
}

func New(scr tcell.Screen, paths config.Paths, client func(llm.Provider) llm.Client) (*App, error) {
	st := config.NewStore(paths)
	s, err := st.LoadSettings()
	if err != nil {
		return nil, err
	}
	gate := jsvm.NewGate(paths.Cwd, s.Permissions, nil)
	a := &App{
		scr: scr, store: st, paths: paths, gate: gate,
		mcp: mcp.NewManager(), client: client, events: make(chan any, 512),
	}
	a.st = NewState()
	a.st.Settings = s
	a.gate.Approve = a.requestApproval
	a.vm = jsvm.New(jsvm.Options{Gate: gate, Timeout: s.TimeoutDuration()})
	if err := a.loadAll(); err != nil {
		return nil, err
	}
	a.st.Session = agent.NewSession(paths.Cwd, s.Model)
	return a, nil
}

func (a *App) loadAll() error {
	var err error
	if a.st.Providers, err = a.store.LoadProviders(); err != nil {
		return err
	}
	if a.st.Skills, err = a.store.LoadSkills(); err != nil {
		return err
	}
	if a.st.Servers, err = a.store.LoadServers(); err != nil {
		return err
	}
	if a.st.Prompts, err = a.store.LoadPrompts(); err != nil {
		return err
	}
	return nil
}

func (a *App) requestApproval(action, target string) bool {
	reply := make(chan bool, 1)
	a.events <- approvalReq{action: action, target: target, reply: reply}
	return <-reply
}

func (a *App) Close() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	a.running.Wait()
	a.mcp.StopAll()
}

func (a *App) Run(ctx context.Context) error {
	go func() {
		for {
			ev := a.scr.PollEvent()
			if ev == nil {
				return
			}
			a.events <- ev
		}
	}()
	a.draw()
	for !a.quit {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-a.events:
			a.handle(msg)
		}
		a.draw()
	}
	return nil
}

func (a *App) handle(msg any) {
	switch m := msg.(type) {
	case *tcell.EventKey:
		a.onKey(m)
	case *tcell.EventResize:
		a.scr.Sync()
	case agentEv:
		a.onAgent(m.ev)
	case turnDone:
		a.onTurnDone(m.err)
	case approvalReq:
		a.st.Pending = &m
	}
}
