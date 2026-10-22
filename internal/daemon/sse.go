package daemon

import (
	"encoding/json"
	"fmt"
	"meno/internal/api"
	"net/http"
	"strconv"
	"time"
)

func parseAfter(r *http.Request) uint64 {
	v := r.Header.Get("Last-Event-ID")
	if v == "" {
		v = r.URL.Query().Get("after")
	}
	n, _ := strconv.ParseUint(v, 10, 64)
	return n
}
func (s *Server) sessionEvents(w http.ResponseWriter, r *http.Request) {
	s.sse(w, r, r.PathValue("id"))
}
func (s *Server) globalEvents(w http.ResponseWriter, r *http.Request) { s.sse(w, r, "") }
func (s *Server) sse(w http.ResponseWriter, r *http.Request, key string) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unavailable", 500)
		return
	}
	var ch <-chan api.Event
	var cancel func()
	var resync bool
	if key == "" {
		ch, cancel, resync = s.global.Subscribe(parseAfter(r), bootFrom(r))
	} else {
		ch, cancel, resync = s.hub.Subscribe(key, parseAfter(r), bootFrom(r))
	}
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	seq := uint64(0)
	if key == "" {
		seq = s.global.Sequence()
	} else {
		seq = s.hub.Sequence(key)
	}
	hello := api.Event{Seq: seq, Type: api.EventHello, BootID: s.bootID, SessionID: key}
	hello.Data, _ = json.Marshal(map[string]any{"boot_id": s.bootID, "seq": hello.Seq})
	writeEvent(w, hello)
	fl.Flush()
	if resync {
		for e := range ch {
			writeEvent(w, e)
		}
		fl.Flush()
		return
	}
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, open := <-ch:
			if !open {
				return
			}
			writeEvent(w, e)
			fl.Flush()
			if e.Type == api.EventResync || e.Type == api.EventBye {
				return
			}
		case <-ping.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		}
	}
}
func bootFrom(r *http.Request) string {
	b := r.URL.Query().Get("boot_id")
	if b == "" {
		b = r.Header.Get("X-Meno-Boot-ID")
	}
	return b
}
func writeEvent(w http.ResponseWriter, e api.Event) {
	data := e.Data
	if len(data) == 0 {
		data = []byte("{}")
	}
	if e.Type == api.EventHello {
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
		return
	}
	_, _ = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Seq, e.Type, data)
}
