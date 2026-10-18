package proc

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Stream names in log records.
const (
	Stdout = "stdout"
	Stderr = "stderr"
)

// Record is one captured chunk of output: a line, or a long line cut at
// MaxRecord bytes. Seq orders records in capture order across both streams;
// that is the order meno read them, not a promise about the order the process
// wrote them.
type Record struct {
	Seq    int64     `json:"seq"`
	Time   time.Time `json:"time"`
	Stream string    `json:"stream"`
	Text   string    `json:"text"`
}

// MaxRecord bounds one record; longer lines are split.
const MaxRecord = 4096

// DefaultLogLimit is the disk budget of one task's log.
const DefaultLogLimit = 10 << 20

type wireRecord struct {
	Q int64  `json:"q"`
	T int64  `json:"t"`
	S string `json:"s"` // "o" stdout, "e" stderr
	D string `json:"d"`
}

// logWriter appends records to two rotating segments, keeping the newest
// data within limit bytes in total.
type logWriter struct {
	mu     sync.Mutex
	path   string
	limit  int64
	f      *os.File
	size   int64
	seq    int64
	closed bool
}

func newLogWriter(path string, limit int64) (*logWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	_ = os.Remove(path + ".1")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	return &logWriter{path: path, limit: max(limit, 4*MaxRecord), f: f}, nil
}

func (w *logWriter) write(stream, text string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	w.seq++
	code := "o"
	if stream == Stderr {
		code = "e"
	}
	b, _ := json.Marshal(wireRecord{w.seq, time.Now().UnixNano(), code, strings.ToValidUTF8(text, "�")})
	b = append(b, '\n')
	if w.size > 0 && w.size+int64(len(b)) > w.limit/2 {
		// Rotate: the older segment is dropped, the current one becomes it.
		_ = w.f.Close()
		_ = os.Remove(w.path + ".1")
		_ = os.Rename(w.path, w.path+".1")
		f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			w.closed = true
			return
		}
		w.f, w.size = f, 0
	}
	n, _ := w.f.Write(b)
	w.size += int64(n)
}

func (w *logWriter) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		w.closed = true
		_ = w.f.Close()
	}
}

// pump copies the lines of r into the log until EOF.
func (w *logWriter) pump(stream string, r *bufio.Reader, done func()) {
	defer done()
	buf := make([]byte, 0, MaxRecord)
	flush := func() {
		if len(buf) > 0 {
			w.write(stream, string(buf))
			buf = buf[:0]
		}
	}
	for {
		b, err := r.ReadByte()
		if err != nil {
			flush()
			return
		}
		if b == '\n' {
			flush()
			continue
		}
		buf = append(buf, b)
		if len(buf) >= MaxRecord {
			flush()
		}
	}
}

// LogOptions select what ReadLogs returns.
type LogOptions struct {
	Cursor int64  // return records after this sequence number
	Tail   bool   // no cursor given: return the newest records
	Stream string // "", "all", "stdout" or "stderr"
	Limit  int    // records, default 200
	Bytes  int    // output bytes, default 64 KiB
}

// LogSlice is a bounded part of a task's log.
type LogSlice struct {
	Records []Record `json:"records"`
	// NextCursor is the sequence number to pass next time.
	NextCursor int64 `json:"nextCursor"`
	// FirstSeq is the oldest record still on disk; Lost counts the records
	// that rotation dropped between the cursor and FirstSeq.
	FirstSeq  int64 `json:"firstSeq"`
	Lost      int64 `json:"lost"`
	Truncated bool  `json:"truncated"`
}

func readSegment(path string) []wireRecord {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []wireRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		var r wireRecord
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	return out
}

// ReadLogs reads the log of a task from disk. A cursor older than the oldest
// retained record starts at the first available one and reports the gap.
func ReadLogs(path string, o LogOptions) LogSlice {
	if o.Limit <= 0 {
		o.Limit = 200
	}
	if o.Bytes <= 0 {
		o.Bytes = 64 << 10
	}
	all := append(readSegment(path+".1"), readSegment(path)...)
	var out LogSlice
	out.Records = []Record{}
	if len(all) == 0 {
		out.NextCursor = o.Cursor
		return out
	}
	out.FirstSeq = all[0].Q
	last := all[len(all)-1].Q
	keep := func(r wireRecord) bool {
		switch o.Stream {
		case Stdout:
			return r.S == "o"
		case Stderr:
			return r.S == "e"
		}
		return true
	}
	var sel []wireRecord
	for _, r := range all {
		if keep(r) && (o.Tail || r.Q > o.Cursor) {
			sel = append(sel, r)
		}
	}
	if !o.Tail && o.Cursor+1 < out.FirstSeq {
		out.Lost = out.FirstSeq - o.Cursor - 1
	}
	if o.Tail {
		if len(sel) > o.Limit {
			sel, out.Truncated = sel[len(sel)-o.Limit:], true
		}
		size := 0
		for i := len(sel) - 1; i >= 0; i-- {
			size += len(sel[i].D)
			if size > o.Bytes && i < len(sel)-1 {
				sel, out.Truncated = sel[i+1:], true
				break
			}
		}
	} else {
		size := 0
		for i, r := range sel {
			size += len(r.D)
			if i >= o.Limit || size > o.Bytes && i > 0 {
				sel, out.Truncated = sel[:i], true
				break
			}
		}
	}
	for _, r := range sel {
		stream := Stdout
		if r.S == "e" {
			stream = Stderr
		}
		out.Records = append(out.Records, Record{Seq: r.Q, Time: time.Unix(0, r.T), Stream: stream, Text: r.D})
	}
	out.NextCursor = last
	if out.Truncated && !o.Tail && len(out.Records) > 0 {
		out.NextCursor = out.Records[len(out.Records)-1].Seq
	}
	return out
}
