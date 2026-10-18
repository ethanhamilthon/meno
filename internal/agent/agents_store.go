package agent

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"meno/internal/llm"
)

// Child agent statuses.
const (
	ChildStarting    = "starting"
	ChildRunning     = "running"
	ChildBlocked     = "blocked"
	ChildCompleted   = "completed"
	ChildFailed      = "failed"
	ChildCancelled   = "cancelled"
	ChildInterrupted = "interrupted"
)

// ChildRecord is the lifecycle of one subagent.
type ChildRecord struct {
	ID          string    `json:"id"`
	Parent      string    `json:"parent"`
	Owner       string    `json:"owner"` // root request of the current execution generation
	Task        string    `json:"task"`
	Status      string    `json:"status"`
	Result      *string   `json:"result"`
	Error       string    `json:"error"`
	BlockReason string    `json:"blockReason,omitempty"`
	Gen         int       `json:"generation"`
	Usage       llm.Usage `json:"usage"`
	Cost        float64   `json:"cost"`
	Unpriced    bool      `json:"unpriced"` // some requests had no known price: Cost is a lower bound
	Created     time.Time `json:"created"`
	Ended       time.Time `json:"ended"`
}

// Working reports whether the child is still doing something for its parent.
func (c ChildRecord) Working() bool {
	return c.Status == ChildStarting || c.Status == ChildRunning || c.Status == ChildBlocked
}

// ChildStore persists subagent lifecycles.
type ChildStore struct{ d *sql.DB }

// OpenChildren opens the subagent table of the meno database under dir.
func OpenChildren(dir string) (*ChildStore, error) {
	d, err := openDB(dir)
	if err != nil {
		return nil, err
	}
	return &ChildStore{d}, nil
}

const childCols = "id,parent_id,owner,task,status,result,error,block_reason,gen,usage,cost,unpriced,created_at,ended_at"

func scanChild(sc interface{ Scan(...any) error }) (ChildRecord, error) {
	var c ChildRecord
	var result sql.NullString
	var usage, created, ended string
	var unpriced int
	err := sc.Scan(&c.ID, &c.Parent, &c.Owner, &c.Task, &c.Status, &result, &c.Error, &c.BlockReason, &c.Gen, &usage, &c.Cost, &unpriced, &created, &ended)
	if err != nil {
		return c, err
	}
	if result.Valid {
		c.Result = &result.String
	}
	c.Unpriced = unpriced != 0
	_ = json.Unmarshal([]byte(usage), &c.Usage)
	c.Created, c.Ended = parseTS(created), parseTS(ended)
	return c, nil
}

// Save inserts or updates a record; the child's session row is created if it
// does not exist yet.
func (s *ChildStore) Save(c ChildRecord) error {
	usage, _ := json.Marshal(c.Usage)
	var result any
	if c.Result != nil {
		result = *c.Result
	}
	ended := ""
	if !c.Ended.IsZero() {
		ended = ts(c.Ended)
	}
	tx, err := s.d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("INSERT OR IGNORE INTO sessions(id,kind,parent_id,updated_at) VALUES(?,?,?,?)", c.ID, KindChild, c.Parent, ts(time.Now())); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO agents("+childCols+") VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) "+
		"ON CONFLICT(id) DO UPDATE SET owner=excluded.owner, status=excluded.status, result=excluded.result, error=excluded.error, "+
		"block_reason=excluded.block_reason, gen=excluded.gen, usage=excluded.usage, cost=excluded.cost, unpriced=excluded.unpriced, ended_at=excluded.ended_at",
		c.ID, c.Parent, c.Owner, c.Task, c.Status, result, c.Error, c.BlockReason, c.Gen, string(usage), c.Cost, b2i(c.Unpriced), ts(c.Created), ended); err != nil {
		return err
	}
	return tx.Commit()
}

// Get returns a child of parent; a child of another parent is not found.
func (s *ChildStore) Get(parent, id string) (ChildRecord, error) {
	c, err := scanChild(s.d.QueryRow("SELECT "+childCols+" FROM agents WHERE id = ? AND parent_id = ?", id, parent))
	if errors.Is(err, sql.ErrNoRows) {
		return c, fmt.Errorf("agent %q not found", id)
	}
	return c, err
}

// List returns the children of a parent, oldest first.
func (s *ChildStore) List(parent string, workingOnly bool) ([]ChildRecord, error) {
	q := "SELECT " + childCols + " FROM agents WHERE parent_id = ?"
	if workingOnly {
		q += " AND status IN ('starting','running','blocked')"
	}
	rows, err := s.d.Query(q+" ORDER BY created_at, id", parent)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChildRecord{}
	for rows.Next() {
		c, err := scanChild(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkInterrupted flags children that were working when meno stopped. They are
// not restarted.
func (s *ChildStore) MarkInterrupted() error {
	_, err := s.d.Exec("UPDATE agents SET status='interrupted', error='meno exited while the agent was working', ended_at=? WHERE status IN ('starting','running','blocked')", ts(time.Now()))
	return err
}
