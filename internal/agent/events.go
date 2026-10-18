package agent

import (
	"database/sql"
	"errors"
	"html"
	"strings"
	"time"

	"meno/internal/llm"
)

// KindEvent marks a user-role turn that carries a steering event rather than
// words typed by the user.
const KindEvent = "event"

// Event sources. Provenance is set by application wiring, never taken from
// text a tool produced.
const (
	SourceUser   = "user"   // the user wrote to a child agent
	SourceParent = "parent" // a parent agent wrote to its child
	SourceChild  = "child"  // a child agent reported, blocked or finished
	SourceTask   = "task"   // a background process finished
	SourceSystem = "system" // meno itself
)

// Event is one item of the steering queue.
type Event struct {
	ID      string
	Session string
	Source  string
	From    string // child or task ID
	Kind    string // message, report, blocked, completed, failed, task_done...
	Owner   string // root request that owns the work the event is about
	Text    string
	Refs    []string // result, log or artifact references
	Time    time.Time
}

// Render is the wrapper the model sees. Every field is escaped, so event
// content can never forge another event or a system block.
func (e Event) Render() string {
	if e.Source == SourceUser {
		// The user speaking to a child directly; the wrapper is the provenance.
		return "<user-message id=\"" + html.EscapeString(e.ID) + "\">\n" + html.EscapeString(e.Text) + "\n</user-message>"
	}
	var b strings.Builder
	b.WriteString("<event id=\"" + html.EscapeString(e.ID) + "\" source=\"" + html.EscapeString(e.Source) + "\"")
	if e.From != "" {
		b.WriteString(" from=\"" + html.EscapeString(e.From) + "\"")
	}
	if e.Kind != "" {
		b.WriteString(" kind=\"" + html.EscapeString(e.Kind) + "\"")
	}
	b.WriteString(">\n" + html.EscapeString(e.Text))
	for _, r := range e.Refs {
		b.WriteString("\nref: " + html.EscapeString(r))
	}
	b.WriteString("\n</event>")
	return b.String()
}

// EventSource hands the runner the events waiting for its session. Taking an
// event marks it delivered, so a restart or a second call never repeats it.
type EventSource interface {
	TakeEvents() []Event
}

// Event states.
const (
	EventPending   = "pending"
	EventDelivered = "delivered"
	EventAcked     = "acked"
	EventCancelled = "cancelled"
)

// EventStore is the durable steering queue.
type EventStore struct{ d *sql.DB }

// OpenEvents opens the queue stored in the meno database under dir.
func OpenEvents(dir string) (*EventStore, error) {
	d, err := openDB(dir)
	if err != nil {
		return nil, err
	}
	return &EventStore{d}, nil
}

// Enqueue persists the event before anything wakes the session. It reports
// false when an event with this ID already exists (a redelivery).
func (s *EventStore) Enqueue(e Event) (bool, error) {
	if e.ID == "" || e.Session == "" || e.Source == "" {
		return false, errors.New("event needs an id, a session and a source")
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	tx, err := s.d.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("INSERT OR IGNORE INTO sessions(id,updated_at) VALUES(?,?)", e.Session, ts(e.Time)); err != nil {
		return false, err
	}
	var seq int64
	if err := tx.QueryRow("SELECT COALESCE(MAX(seq),0)+1 FROM events WHERE session_id = ?", e.Session).Scan(&seq); err != nil {
		return false, err
	}
	res, err := tx.Exec("INSERT OR IGNORE INTO events(session_id,id,seq,source,from_id,kind,owner,text,refs,time) VALUES(?,?,?,?,?,?,?,?,?,?)",
		e.Session, e.ID, seq, e.Source, e.From, e.Kind, e.Owner, e.Text, strings.Join(e.Refs, "\n"), ts(e.Time))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, tx.Commit()
}

type queryer interface {
	Query(string, ...any) (*sql.Rows, error)
}

func selectEvents(q queryer, session string) ([]Event, error) {
	rows, err := q.Query("SELECT id,source,from_id,kind,owner,text,refs,time FROM events WHERE session_id = ? AND state = 'pending' ORDER BY seq", session)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		e := Event{Session: session}
		var refs, when string
		if err := rows.Scan(&e.ID, &e.Source, &e.From, &e.Kind, &e.Owner, &e.Text, &refs, &when); err != nil {
			return nil, err
		}
		if refs != "" {
			e.Refs = strings.Split(refs, "\n")
		}
		e.Time = parseTS(when)
		out = append(out, e)
	}
	return out, rows.Err()
}

// Pending lists the events waiting for a session, oldest first.
func (s *EventStore) Pending(session string) ([]Event, error) {
	return selectEvents(s.d, session)
}

// Take returns the pending events and marks them delivered in one transaction.
func (s *EventStore) Take(session string) ([]Event, error) {
	tx, err := s.d.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	evs, err := selectEvents(tx, session)
	if err != nil || len(evs) == 0 {
		return nil, err
	}
	if _, err := tx.Exec("UPDATE events SET state = 'delivered' WHERE session_id = ? AND state = 'pending'", session); err != nil {
		return nil, err
	}
	return evs, tx.Commit()
}

// Ack records that the model has seen the outcome through an explicit wait, so
// a pending notification for it is not delivered again.
func (s *EventStore) Ack(session, id string) error {
	_, err := s.d.Exec("UPDATE events SET state = 'acked' WHERE session_id = ? AND id = ? AND state = 'pending'", session, id)
	return err
}

// CancelOwner drops the pending events of a cancelled request.
func (s *EventStore) CancelOwner(session, owner string) error {
	_, err := s.d.Exec("UPDATE events SET state = 'cancelled' WHERE session_id = ? AND owner = ? AND state = 'pending'", session, owner)
	return err
}

// State returns the state of an event ("" when unknown).
func (s *EventStore) State(session, id string) string {
	var st string
	_ = s.d.QueryRow("SELECT state FROM events WHERE session_id = ? AND id = ?", session, id).Scan(&st)
	return st
}

// StoreSource adapts the store to the runner for one session.
type StoreSource struct {
	Store   *EventStore
	Session string
}

func (s StoreSource) TakeEvents() []Event {
	evs, _ := s.Store.Take(s.Session)
	return evs
}

func eventTurn(e Event) Turn {
	return Turn{Role: llm.RoleUser, Kind: KindEvent, Text: e.Render(), EventID: e.ID, Time: nowUTC()}
}
