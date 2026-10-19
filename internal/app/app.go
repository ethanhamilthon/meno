package app

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/db"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/proc"
	"meno/internal/safe"
	"meno/internal/ui"
)

type App struct {
	scr        tcell.Screen
	drawErr    string   // last rendering panic, reported once until it changes
	warnings   []string // problems found in provider.json, mcp.json, theme.json at start
	mouse      mouseState
	st         *State
	store      *config.Store
	paths      config.Paths
	mcp        *mcp.Manager
	mcpCtx     context.Context // lives as long as the app; ends every server process
	mcpStop    context.CancelFunc
	retry      agent.RetryPolicy // how failed model requests are repeated; the zero value is the default
	client     func(llm.Provider) llm.Client
	events     chan any
	quit       bool
	done       chan struct{}
	closeOnce  sync.Once
	waveOn     atomic.Bool    // the welcome screen was drawn last: keep it animating
	agentsAt   time.Time      // when agentsOK was last checked
	agentsOK   bool           // an AGENTS.md exists (cached for the welcome screen)
	idleTick   bool           // the batch held only ticks: skip drawing when nothing runs
	bg         map[string]int // running tasks and working subagents per session id
	bgAt       time.Time      // when the background work was last counted
	bgInflight bool           // a count is running
	cur        cursorPos      // real terminal cursor, set while drawing

	imgs    imageMgr          // inline terminal graphics
	evStore *agent.EventStore // the durable steering queue
	tasks   *proc.Supervisor  // background processes of the main agents
	daemons *proc.Daemons     // the run commands of providers (local proxies)
	kids    *childMgr         // subagents of the main sessions

	loginParts // the OAuth helper and the login in progress (login_oauth.go)

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
	// checkUpdate looks for a newer release (see SetUpdateCheck); nil: no check.
	checkUpdate func(ctx context.Context) string
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
	a.openURL = openBrowser
	a.evStore, _ = agent.OpenEvents(paths.Home)
	if d, err := db.Open(paths.Home); err == nil {
		a.tasks, _ = proc.New(d, filepath.Join(paths.Home, "tasks"), proc.Hooks{Finished: a.onTaskFinished})
	}
	a.kids = newChildMgr(a)
	a.daemons = proc.NewDaemons(filepath.Join(paths.Home, "providers"))
	a.daemons.OnExit = func(name string, err error, log string) {
		if a.helperStopped(name, err, log) {
			return
		}
		a.postEvent(errNotice{text: "provider " + name + ": the run command ended (" + err.Error() + "), see " + log})
	}
	a.mcpCtx, a.mcpStop = context.WithCancel(context.Background())
	a.st = NewState()
	a.st.Settings = s
	a.loadThemeFile()
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
	a.reportWarnings()
	return a, nil
}

func (a *App) Close() {
	a.cancelFlow()
	a.clearImages()
	a.closeOnce.Do(func() { close(a.done) })
	a.mcpStop()
	a.cancelAll()
	a.running.Wait()
	if a.kids != nil {
		a.kids.Shutdown()
	}
	if a.daemons != nil {
		a.daemons.StopAll() // the proxies of the providers go with meno
	}
	if a.tasks != nil {
		a.tasks.Shutdown() // cancels every managed process group before the database closes
	}
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
	a.startUpdateCheck()
	a.startProviderRuns()
	a.startOAuth()
	a.enforceProvider()
	a.startServers()
	safe.Go(func() {
		for {
			ev := a.scr.PollEvent()
			if ev == nil {
				return
			}
			a.postEvent(ev)
		}
	}, a.reportPanic)
	safe.Go(a.ticker, a.reportPanic)
	safe.Go(a.waveTicker, a.reportPanic)
	a.drawSafe()
	for !a.quit {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-a.events:
			a.idleTick = true
			a.dispatch(msg)
			// Coalesce bursts (wheel scroll, streaming tokens, paste) into one frame.
		drain:
			for n := 0; n < 256 && !a.quit; n++ {
				select {
				case msg = <-a.events:
					a.dispatch(msg)
				default:
					break drain
				}
			}
		}
		if !a.idleTick || a.animating() {
			a.drawSafe()
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
	case internalPanic:
		a.notifyErr("internal error, meno keeps running: " + strings.SplitN(m.err.Error(), "\n", 2)[0])
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
		a.onMouse(m)
	case *tcell.EventResize:
		// tcell already resized and invalidated its buffer; a full Sync() clears
		// the terminal and causes visible flicker.
	case agentEv:
		a.inRT(m.sid, func() { a.onAgent(m.ev) })
	case imageDecoded:
		a.imgs.placed = ""
	case pickResults:
		a.onPickResults(m)
	case errNotice:
		a.notifyErr(m.text)
	case updateAvailable:
		a.onUpdateAvailable(m.tag)
	case tasksLoaded:
		a.onTasksLoaded(m)
	case bgLoaded:
		a.onBgLoaded(m)
	case childChanged:
		a.onChildChanged(m.parent)
	case eventQueued:
		a.onEventQueued(m.sid)
	case sessionsLoaded:
		a.onSessionsLoaded(m)
	case modelsLoaded:
		a.onModelsLoaded(m)
	case accountsLoaded, oauthPortChanged, oauthHelperStatus, oauthRefreshed, flowStep, flowLogin, flowDone:
		a.onLoginEvent(m)
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
	case routingStateEv, preparedEv:
		a.handleRoutingEvent(m)
	case turnDone:
		a.inRT(m.sid, func() {
			a.onGoalEnd(m.err) // before onTurnDone, which may start the next request and replace the goal
			a.onTurnDone(m.err)
		})
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
	case waveTick:
	case tick:
		a.refreshTasks()
		a.refreshBackground()
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
