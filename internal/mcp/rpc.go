package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"meno/internal/safe"
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
	notify  func(string)
}

func newConn(w io.Writer) *conn { return &conn{w: w} }

func (c *conn) send(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	n, err := c.w.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return err
}

func (c *conn) call(method string, params any) (json.RawMessage, chan rpcMsg, int64, error) {
	id := c.nextID.Add(1)
	ch := make(chan rpcMsg, 1)
	c.pending.Store(id, ch)
	err := c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		c.pending.Delete(id)
	}
	return nil, ch, id, err
}

// handle routes one JSON-RPC message from the server: a response goes to the
// call waiting for it; notifications are forwarded and requests receive replies.
func (c *conn) handle(line []byte) {
	var m rpcMsg
	var head struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if json.Unmarshal(line, &head) != nil {
		return
	}
	if len(head.ID) == 0 || string(head.ID) == "null" {
		if c.notify != nil && head.Method != "" {
			c.notify(head.Method)
		}
		return
	}
	if head.Method != "" {
		reply := map[string]any{"jsonrpc": "2.0", "id": head.ID}
		if head.Method == "ping" {
			reply["result"] = map[string]any{}
		} else {
			reply["error"] = map[string]any{"code": -32601, "message": "method not supported"}
		}
		_ = c.send(reply)
		return
	}
	if json.Unmarshal(line, &m) != nil || m.ID == nil {
		return
	}
	if m.Result == nil && m.Error == nil {
		return
	}
	if v, ok := c.pending.LoadAndDelete(*m.ID); ok {
		v.(chan rpcMsg) <- m
	}
}

// fail completes a pending call with a transport error.
func (c *conn) fail(id int64, err error) {
	if v, ok := c.pending.LoadAndDelete(id); ok {
		v.(chan rpcMsg) <- rpcMsg{ID: &id, Error: &rpcError{Code: -32000, Message: err.Error()}}
	}
}

func (c *conn) dispatch(r io.Reader) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
	for sc.Scan() {
		// One malformed or unexpected message from a server must not kill the process.
		_ = safe.Call(func() { c.handle(sc.Bytes()) })
	}
	return sc.Err()
}
