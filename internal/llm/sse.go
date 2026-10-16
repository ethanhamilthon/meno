package llm

import (
	"bufio"
	"io"
	"strings"
)

type SSEEvent struct {
	Event string
	Data  string
}

func ReadSSE(r io.Reader, fn func(SSEEvent) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var ev SSEEvent
	var data []string
	flush := func() error {
		if len(data) == 0 && ev.Event == "" {
			return nil
		}
		ev.Data = strings.Join(data, "\n")
		err := fn(ev)
		ev = SSEEvent{}
		data = nil
		return err
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		k, v, _ := strings.Cut(line, ":")
		v = strings.TrimPrefix(v, " ")
		switch k {
		case "event":
			ev.Event = v
		case "data":
			data = append(data, v)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return flush()
}
