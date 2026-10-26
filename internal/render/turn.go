package render

// Turn kinds and roles the transcript knows. They match the daemon wire values.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	KindEvent     = "event"
	KindView      = "view"
	KindSkill     = "skill"
	RunFail       = "fail"
	RunStop       = "stop"
)

// RunStats is what one run took; it closes the run in the transcript.
type RunStats struct {
	Status          string
	ElapsedMs       int64
	Turns           int
	ToolCalls       int
	ToolMs          int64
	In, Out, Cached int
	CacheWrite      int
	CacheKnown      bool
	Partial         bool
	Cost            *float64
	Err             string
}

// Turn is one item of the transcript as the view draws it.
type Turn struct {
	ID        string // stable id; turns with an id are laid out once and cached
	CallID    string
	Role      string
	Kind      string
	Text      string
	Thinking  string
	Code      string
	Title     string
	Output    string
	Err       string
	ElapsedMs int64
	LimitMs   int64
	TimedOut  bool
	Run       *RunStats
}
