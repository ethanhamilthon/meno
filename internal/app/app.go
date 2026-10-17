package app

import (
	"context"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/ui"
)

type App struct {
	scr       tcell.Screen
	st        *State
	store     *config.Store
	paths     config.Paths
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

	rts    map[string]*Runtime // every session loaded in this run, by id
	rtList []*Runtime          // the same, in creation order
	dbRows []agent.SessionInfo // sessions in the database, as last listed for :sessions

	closing bool     // Close is running: every turn has ended, so even a "running" session may be saved
	cdKey   string   // input and directory the :cd completions in cdCache belong to
	cdCache []string // rows of the :cd completion

	mu sync.Mutex // guards Runtime.cancel
	// prepareHook, when set, runs first in the goroutine that prepares a turn (tests slow it down).
	prepareHook func()
	running     sync.WaitGroup

	pasting  bool   // between the start and end of a bracketed paste into the composer
	pasteBuf []rune // text collected during that paste
	// clipboard reads an image from the system clipboard (tests replace it).
	clipboard func() ([]byte, error)
	// notifier replaces the way signals are made (tests).
	notifier func(cmd string, env []string)
}

type agentEv struct {
	sid string
	ev  agent.Ev
}
type turnDone struct {
	sid string
	err error
}

// preparedEv carries the prompt state and system event of a message that is
// about to be sent.
type preparedEv struct {
	sid string
	p   prepared
}

// tick drives animations (the blinking activity dot) while the agent works.
type tick struct{}

type compactDone struct {
	sid           string
	before, after int
	err           error
}
type sessionsLoaded struct {
	list []agent.SessionInfo
	err  error
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
		rts: map[string]*Runtime{},
	}
	a.mcpCtx, a.mcpStop = context.WithCancel(context.Background())
	a.st = NewState()
	a.st.Settings = s
	if !ui.SetTheme(s.Theme) {
		ui.SetTheme(ui.DefaultTheme)
	}
	a.loadMetadataCache()
	if err := a.loadAll(); err != nil {
		return nil, err
	}
	// One empty session in the launch directory; sessions of earlier runs are
	// only opened on request (:sessions).
	a.st.Runtime = a.newRuntime(a.newAgentSession(paths.Cwd), paths.Cwd)
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
	a.cancelAll()
	a.running.Wait()
	a.closing = true
	for _, rt := range a.rtList {
		a.saveRT(rt)
	}
	agent.CloseSessionStores()
	a.mcp.StopAll()
}

func (a *App) Run(ctx context.Context) error {
	a.scr.EnablePaste()
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
		if !a.idleTick || a.animating() {
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
		if a.pasting {
			a.collectPaste(m)
			return
		}
		a.onKey(m)
	case *tcell.EventPaste:
		a.onPaste(m)
	case imageReady:
		a.onImageReady(m)
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
		a.inRT(m.sid, func() { a.onAgent(m.ev) })
	case sessionsLoaded:
		a.onSessionsLoaded(m)
	case modelsLoaded:
		a.onModelsLoaded(m)
	case compactDone:
		a.inRT(m.sid, func() {
			if m.err == nil {
				a.st.Prompt.Base, a.st.Prompt.Frozen = nil, "" // a reset point: the cache is gone anyway
				a.st.Context = m.after
				a.notify("context compacted · " + itoa(m.before) + " → " + itoa(m.after) + " tokens")
			}
			a.onTurnDone(m.err)
		})
		a.markUnseen(m.sid)
	case preparedEv:
		a.inRT(m.sid, func() { a.onPrepared(m.p) })
	case turnDone:
		a.inRT(m.sid, func() { a.onTurnDone(m.err) })
		a.markUnseen(m.sid)
	case askOpen:
		a.inRT(m.sid, func() { a.openAsk(m.st) })
	case askClose:
		a.inRT(m.sid, func() { a.closeAsk(m.st) })
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
