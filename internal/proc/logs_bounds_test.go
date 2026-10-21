package proc

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func logSize(path string) int64 {
	var n int64
	for _, p := range []string{path, path + ".1"} {
		if st, err := os.Stat(p); err == nil {
			n += st.Size()
		}
	}
	return n
}

// Control characters grow 6x when JSON-escaped. The disk budget must hold for
// escaped bytes, not only for the raw bytes the process wrote.
func TestEscapedRecordsStayWithinTheDiskBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task.log")
	const limit = 32 << 10
	w, err := newLogWriter(path, limit)
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()
	line := strings.Repeat("\x01", MaxRecord) + "\n"
	for i := 0; i < 200; i++ {
		var done bool
		w.pump(Stdout, bufio.NewReader(bytes.NewReader([]byte(line))), func() { done = true })
		if !done {
			t.Fatal("pump must finish")
		}
	}
	if got := logSize(path); got > limit {
		t.Fatalf("log uses %d bytes, budget %d", got, limit)
	}
}

// A line without a newline must be cut into bounded records, and paging
// through them must make progress and end.
func TestLongLinesWithoutNewlineAreBoundedAndPageForward(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task.log")
	w, _ := newLogWriter(path, DefaultLogLimit)
	defer w.close()
	w.pump(Stderr, bufio.NewReader(strings.NewReader(strings.Repeat("x", 10*MaxRecord))), func() {})
	var cursor int64
	total := 0
	for i := 0; i < 100; i++ {
		s := ReadLogs(path, LogOptions{Cursor: cursor, Limit: 3})
		for _, r := range s.Records {
			if len(r.Text) > MaxRecord {
				t.Fatalf("record of %d bytes", len(r.Text))
			}
			total += len(r.Text)
		}
		if s.NextCursor == cursor {
			break
		}
		cursor = s.NextCursor
	}
	if total != 10*MaxRecord {
		t.Fatalf("paged %d bytes, want %d", total, 10*MaxRecord)
	}
}
