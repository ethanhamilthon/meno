package agent

import (
	"fmt"
	"unicode/utf8"
)

// ReadOptions bound a read of one session's history.
type ReadOptions struct {
	From                               int64 // history ID, inclusive; 0 = beginning
	Limit                              int
	IncludeToolOutput, IncludeThinking bool
}

// Record is a message as sessions.read returns it.
type Record struct {
	ID       int64  `json:"id"`
	Role     string `json:"role"`
	Time     string `json:"time"`
	Text     string `json:"text"`
	Code     string `json:"code,omitempty"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	Thinking string `json:"thinking,omitempty"`
	// Omitted lists fields left out by default or for size: name -> bytes.
	Omitted map[string]int `json:"omitted,omitempty"`
	Images  []string       `json:"images,omitempty"`
}

// ReadResult is a bounded slice of a session's history.
type ReadResult struct {
	Session string   `json:"session"`
	Title   string   `json:"title"`
	Records []Record `json:"records"`
	// Next is the history ID to continue from, 0 at the end.
	Next int64 `json:"next"`
}

// ReadSessionHistory returns a bounded slice of the complete history of a
// session. Thinking and tool output are omitted unless asked for; omitted
// fields are reported with their size, never shown as empty.
func ReadSessionHistory(dir, id string, o ReadOptions) (ReadResult, error) {
	d, err := openDB(dir)
	if err != nil {
		return ReadResult{}, err
	}
	var title string
	if err := d.QueryRow("SELECT title FROM sessions WHERE id = ?", id).Scan(&title); err != nil {
		return ReadResult{}, fmt.Errorf("session %q not found", id)
	}
	limit := o.Limit
	if limit <= 0 || limit > 500 {
		limit = DefaultRead
	}
	after := max(o.From-1, 0)
	turns, err := readHistory(d, id, after, limit+1)
	if err != nil {
		return ReadResult{}, err
	}
	res := ReadResult{Session: id, Title: title, Records: []Record{}}
	budget := ReadByteLimit
	for i, t := range turns {
		if i == limit {
			res.Next = t.HID
			break
		}
		r := Record{ID: t.HID, Role: t.Role, Time: ts(t.Time), Text: t.Text, Code: t.Code, Error: t.Err, Omitted: map[string]int{}}
		for _, a := range t.Attachments {
			r.Images = append(r.Images, "attachment:"+a.SHA)
		}
		if o.IncludeToolOutput {
			r.Output = t.Output
		} else if t.Output != "" {
			r.Omitted["output"] = len(t.Output)
		}
		if o.IncludeThinking {
			r.Thinking = t.Thinking
		} else if t.Thinking != "" {
			r.Omitted["thinking"] = len(t.Thinking)
		}
		size := len(r.Text) + len(r.Code) + len(r.Output) + len(r.Thinking) + len(r.Error)
		if size > budget {
			if len(res.Records) > 0 {
				res.Next = t.HID
				break
			}
			r.Text = cutUTF8(r.Text, budget) // one oversized record: show what fits
			r.Omitted["text"] = len(t.Text) - len(r.Text)
		}
		budget -= min(size, budget)
		if len(r.Omitted) == 0 {
			r.Omitted = nil
		}
		res.Records = append(res.Records, r)
	}
	return res, nil
}

func cutUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
