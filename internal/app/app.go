package app

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/proto"
	"meno/internal/safe"
	"meno/internal/ui"
)

type App struct {
	lastFrame         frameState
	unfocused         bool
	mcpPending        bool
	mcpRefresh        time.Time
	mcpWarning        string
	commandFailed     bool
	globalWatch       globalWatchState
	globalSessions    []proto.SessionItem
	globalTasks       int
	ackPending        bool
	completionVisible bool

	scr       tcell.Screen
	drawErr   string   // last rendering panic, reported once until it changes
	warnings  []string // problems found in the theme file at start
	mouse     mouseState
	st        *State
	paths     config.Paths
	cl        *client.Client // the daemon, which owns sessions, tools, tasks and config
	events    chan any
	quit      bool
	done      chan struct{}
	closeOnce sync.Once
	waveOn    atomic.Bool      // the welcome screen was drawn last: keep it animating
	agentsAt  time.Time        // when agentsOK was last checked
	agentsOK  bool             // an AGENTS.md exists (cached for the welcome screen)
	now       func() time.Time // the clock of animations; tests replace it
	waveEpoch time.Time        // start of the header wave clock
	idleTick  bool             // the batch held only ticks: skip drawing when nothing runs
	cur       cursorPos        // real terminal cursor, set while drawing

	imgs imageMgr // inline terminal graphics

	watchGeneration uint64
	rts             map[string]*Runtime // every session loaded in this run, by id
	rtList          []*Runtime          // the same, in creation order
	dbRows          []proto.SessionItem // sessions the daemon listed for /sessions
	openURL         func(string) error

	saves                saveQueue       // saves to the daemon, one at a time and in order
	cfgSeq               atomic.Uint64   // numbers the reads of the daemon config
	connLost             map[string]bool // sessions whose daemon stream broke and has not recovered
	instance             string          // identity of this window for draft files
	draftSaved           string
	draftFailed          bool
	stopAllAsk           time.Time // when /stop-all asked for confirmation
	cfgPending, cfgDirty bool
	cfgSeen              uint64                  // the newest read that was applied
	cfgBase              config.Settings         // settings as the daemon last reported them: the base of field patches
	win                  windowState             // what belongs to this window only: setting overrides, status-bar layout
	cfgProv              map[string]llm.Provider // own providers as the daemon last reported them
	flowSeq              int                     // number of the last login, to tell a stale one from the current
	closing              bool                    // Close is running
	cdKey                string                  // input and directory the /cd completions in cdCache belong to
	cdCache              []string                // rows of the /cd completion

	running  sync.WaitGroup // short background calls
	watchers sync.WaitGroup // the event streams of followed sessions
	pasting  bool           // between the start and end of a bracketed paste into the composer
	pasteBuf []rune         // text collected during that paste
	// clipboard reads an image from the system clipboard (tests replace it).
	clipboard func() ([]byte, error)
	// notifier replaces the way signals are made (tests).
	notifier func(cmd string, env []string)
	// checkUpdate looks for a newer release (see SetUpdateCheck); nil: no check.
	checkUpdate func(ctx context.Context) string
}

// New builds the TUI on a connection to the daemon. It does not start the
// daemon: the caller (cmd/meno) ensures one is running and passes the client.
func New(scr tcell.Screen, paths config.Paths, cl *client.Client) (*App, error) {
	a := &App{
		scr: scr, paths: paths, cl: cl, done: make(chan struct{}),
		events: make(chan any, 512), rts: map[string]*Runtime{},
	}
	a.openURL = openBrowser
	a.instance = newInstanceID()
	a.waveEpoch = time.Now()
	a.st = NewState()
	scr.EnableFocus()
	a.loadThemeFile()
	if err := a.loadAll(); err != nil {
		return nil, err
	}
	if !ui.SetTheme(a.st.Settings.Theme) {
		ui.SetTheme(ui.DefaultTheme)
	}
	sess, err := a.createSession(paths.Cwd)
	if err != nil {
		return nil, err
	}
	a.st.Runtime = a.newRuntime(sess, paths.Cwd)
	a.st.Runtime.watch = a.watch(sess.ID())
	a.reportWarnings()
	a.announceDrafts()
	a.startGlobalWatch()
	return a, nil
}

// Close stops the TUI. Work in the daemon goes on: closing a client never
// stops a run or a task.
func (a *App) Close() {
	a.closing = true
	a.saveDraft() // the last edits are on disk before the window goes away
	a.cancelFlow()
	a.clearImages()
	a.closeOnce.Do(func() { close(a.done) })
	for _, rt := range a.rtList {
		if rt.watch != nil {
			rt.watch()
		}
	}
	if a.globalWatch.stop != nil {
		a.globalWatch.stop()
	}
	a.running.Wait()
	a.watchers.Wait()
}

func (a *App) Run(ctx context.Context) error {
	a.scr.EnablePaste()
	a.fetchModels()
	a.startUpdateCheck()
	a.refreshConfig()
	a.enforceProvider()
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
	case *tcell.EventFocus:
		a.unfocused = !m.Focused
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
	case *tcell.EventMouse:
		a.onMouse(m)
	case *tcell.EventResize:
		// tcell already resized and invalidated its buffer; a full Sync() clears
		// the terminal and causes visible flicker.
	case projectionEv:
		a.onProjection(m)
	case wireLost:
		if rt := a.rts[m.sid]; rt != nil && rt.generation == m.generation {
			a.setConnLost(m.sid, true)
			a.noticeFor(rt, "lost the daemon stream of a session: "+m.err.Error(), true, false)
		}
	case imageDecoded:
		a.imgs.placed = ""
	case pickResults:
		a.onPickResults(m)
	case exportDone:
		a.onExportDone(m)
	case errNotice:
		a.notifyErr(m.text)
	case updateAvailable:
		a.onUpdateAvailable(m.tag)
	case globalRetry:
		if m.generation == a.globalWatch.generation {
			a.invalidateGlobal()
		}
	case globalWatchEvent:
		a.onGlobalWatch(m)
	case globalSessionsLoaded:
		a.onGlobalSessions(m)
	case completionRead:
		a.ackPending = false
		if m.err == nil {
			if rt := a.rts[m.sid]; rt != nil && rt.Session != nil {
				rt.Session.ReadThrough = max(rt.Session.ReadThrough, m.through)
				rt.Unseen = rt.Session.Completed > rt.Session.ReadThrough
			}
		}
	case sessionsLoaded:
		a.onSessionsLoaded(m)
	case modelsLoaded:
		a.onModelsLoaded(m)
	case settingsLoaded:
		a.onSettingsLoaded(m)
	case accountsLoaded, flowStep, flowLogin, flowDone:
		a.onLoginEvent(m)
	case forked:
		a.onForked(m)
	case tasksLoaded:
		a.onTasksLoaded(m)
	case skillsLoaded:
		a.st.Skills = m.skills
	case compactDone:
		a.onCompactDone(m)
	case sendResult:
		a.onSendResult(m)
	case uiCall:
		m.fn()
		close(m.done)
	case mcpStatusLoaded:
		a.mcpPending = false
		if warnings := strings.Join(m.warnings, "\n"); warnings != a.mcpWarning {
			a.mcpWarning = warnings
			if warnings != "" {
				a.notifyErr(warnings)
			}
		}
		if m.err == nil {
			for _, v := range m.servers {
				a.st.ServerInfo[v.Name] = mcpInfo{Running: v.Running, Starting: v.Starting, Err: v.Error}
			}
		}
	case waveTick:
	case tick:
		a.saveDraft()
		a.refreshMCPStatus()
		a.refreshTasks()
		if a.pruneNotices() {
			a.idleTick = false
		}
	}
}
