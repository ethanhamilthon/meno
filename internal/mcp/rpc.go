package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("rpc %d: %s", e.Code, e.Message) }

type conn struct {
	w       io.Writer
	mu      sync.Mutex
	nextID  atomic.Int64
	pending sync.Map
}

func newConn(w io.Writer) *conn { return &conn{w: w} }

func (c *conn) send(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = c.w.Write(append(b, '\n'))
	return err
}

func (c *conn) call(method string, params any) (json.RawMessage, chan rpcMsg, int64, error) {
	id := c.nextID.Add(1)
	ch := make(chan rpcMsg, 1)
	c.pending.Store(id, ch)
	err := c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	return nil, ch, id, err
}

func (c *conn) dispatch(r io.Reader) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
	for sc.Scan() {
		var m rpcMsg
		if json.Unmarshal(sc.Bytes(), &m) != nil || m.ID == nil || (m.Result == nil && m.Error == nil) {
			continue
		}
		if v, ok := c.pending.LoadAndDelete(*m.ID); ok {
			v.(chan rpcMsg) <- m
		}
	}
	return sc.Err()
}
