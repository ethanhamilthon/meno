package app

import (
	"context"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/jsvm"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/ui"
)

type App struct {
	scr       tcell.Screen
	st        *State
	store     *config.Store
	paths     config.Paths
	vm        *jsvm.VM
	mcp       *mcp.Manager
	mcpCtx    context.Context // lives as long as the app; ends every server process
	mcpStop   context.CancelFunc
	retry     agent.RetryPolicy // how failed model requests are repeated; the zero value is the default
	client    func(llm.Provider) llm.Client
	events    chan any
	quit      bool
	done      chan struct{}
	closeOnce sync.Once
	idleTick  bool      // the batch held only ticks: skip drawing when nothing runs
	cur       cursorPos // real terminal cursor, set while drawing

	mu     sync.Mutex
	cancel context.CancelFunc
	// prepareHook, when set, runs first in the goroutine that prepares a turn (tests slow it down).
	prepareHook func()
	running     sync.WaitGroup
}

type agentEv struct{ ev agent.Ev }
type turnDone struct{ err error }

// preparedEv carries the prompt state and system event of a message that is
// about to be sent.
type preparedEv struct{ p prepared }

// tick drives animations (the blinking activity dot) while the agent works.
type tick struct{}
type compactDone struct {
	before, after int
	err           error
}
type modelsLoaded struct {
	provider string
	models   []llm.Model
	err      error
}

func New(scr tcell.Screen, paths config.Paths, client func(llm.Provider) llm.Client) (*App, error) {
	st := config.NewStore(paths)
	s, err := st.LoadSettings()
	if err != nil {
		return nil, err
	}
	a := &App{
		scr: scr, store: st, paths: paths, done: make(chan struct{}),
		mcp: mcp.NewManager(), client: client, events: make(chan any, 512),
	}
	a.mcpCtx, a.mcpStop = context.WithCancel(context.Background())
	a.st = NewState()
	a.st.Settings = s
	if !ui.SetTheme(s.Theme) {
		ui.SetTheme(ui.DefaultTheme)
	}
	a.loadMetadataCache()
	a.vm = jsvm.New(jsvm.Options{Root: paths.Cwd, Timeout: s.TimeoutDuration()})
	a.vm.SetMCP(mcpHost{a.mcp})
	a.vm.SetMCPAdmin(mcpAdmin{a})
	a.vm.SetAsker(appAsker{a})
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
	return nil
}

func (a *App) Close() {
	a.closeOnce.Do(func() { close(a.done) })
	a.mcpStop()
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	a.running.Wait()
	a.saveSession()
	agent.CloseSessionStores()
	a.mcp.StopAll()
}

func (a *App) Run(ctx context.Context) error {
	a.fetchMetadata()
	a.startServers()
	go func() {
		for {
			ev := a.scr.PollEvent()
			if ev == nil {
				return
			}
			a.postEvent(ev)
		}
	}()
	go a.ticker()
	a.draw()
	for !a.quit {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-a.events:
			a.idleTick = true
			a.handle(msg)
			// Coalesce bursts (wheel scroll, streaming tokens, paste) into one frame.
		drain:
			for n := 0; n < 256 && !a.quit; n++ {
				select {
				case msg = <-a.events:
					a.handle(msg)
				default:
					break drain
				}
			}
		}
		if !a.idleTick || a.st.Running {
			a.draw()
		}
	}
	return nil
}

func (a *App) handle(msg any) {
	if _, ok := msg.(tick); !ok {
		a.idleTick = false
	}
	switch m := msg.(type) {
	case *tcell.EventKey:
		a.onKey(m)
	case metadataLoaded:
		if m.err != nil {
			if len(a.st.Pricing) == 0 && len(a.st.Notices) == 0 {
				a.notifyErr("model metadata: " + m.err.Error())
			}
		} else {
			a.st.Pricing = m.pricing
		}
	case *tcell.EventMouse:
		if a.st.Over == OverNone {
			b := m.Buttons()
			if b&tcell.WheelUp != 0 {
				a.scrollBy(3)
			}
			if b&tcell.WheelDown != 0 {
				a.scrollBy(-3)
			}
		}
	case *tcell.EventResize:
		// tcell already resized and invalidated its buffer; a full Sync() clears
		// the terminal and causes visible flicker.
	case agentEv:
		a.onAgent(m.ev)
	case modelsLoaded:
		a.onModelsLoaded(m)
	case compactDone:
		if m.err == nil {
			a.st.Prompt.Base, a.st.Prompt.Frozen = nil, "" // a reset point: the cache is gone anyway
			a.st.Context = m.after
			a.notify("context compacted · " + itoa(m.before) + " → " + itoa(m.after) + " tokens")
		}
		a.onTurnDone(m.err)
	case preparedEv:
		a.onPrepared(m.p)
	case turnDone:
		a.onTurnDone(m.err)
	case askOpen:
		a.openAsk(m.st)
	case askClose:
		a.closeAsk(m.st)
	case uiCall:
		m.fn()
		close(m.done)
	case mcpStarted:
		if m.err != nil {
			a.notifyErr("mcp " + m.name + ": " + m.err.Error())
		}
	case tick:
		if a.pruneNotices() {
			a.idleTick = false
		}
	}
}

// ticker wakes the render loop twice a second while the agent is running.
func (a *App) ticker() {
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-t.C:
			a.postEvent(tick{})
		}
	}
}
