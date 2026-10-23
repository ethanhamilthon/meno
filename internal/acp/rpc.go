// Package acp serves the Agent Client Protocol (ACP) over a byte stream. It
// translates ACP requests into calls on the daemon client and daemon events
// into ACP session updates. It knows no client product and holds no agent loop,
// storage or terminal code.
package acp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// MaxLineBytes bounds one incoming JSON-RPC message.
const MaxLineBytes = 16 << 20

// JSON-RPC error codes used by the adapter.
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeNoMethod       = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
)

// rpcError is a JSON-RPC error object; handlers return it to choose the code.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string { return e.Message }

func errParams(format string, a ...any) error {
	return &rpcError{Code: codeInvalidParams, Message: fmt.Sprintf(format, a...)}
}

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// handler serves one request or notification. A returned value becomes the result.
type handler func(params json.RawMessage) (any, error)

// afterReply is a handler result that runs then once the reply is written, so
// a notification can follow its response in order on the wire.
type afterReply struct {
	value any
	then  func()
}

// conn is a newline-delimited JSON-RPC 2.0 connection. Writes are serialized.
type conn struct {
	out io.Writer
	wmu sync.Mutex

	mu       sync.Mutex
	inflight map[string]bool
	wg       sync.WaitGroup
}

func newConn(out io.Writer) *conn {
	return &conn{out: out, inflight: map[string]bool{}}
}

func (c *conn) write(m message) error {
	m.JSONRPC = "2.0"
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.out.Write(b)
	return err
}

// Notify sends a notification to the client.
func (c *conn) Notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(message{Method: method, Params: raw})
}

func (c *conn) reply(id json.RawMessage, result any, err error) {
	if err != nil {
		var re *rpcError
		if !errors.As(err, &re) {
			re = &rpcError{Code: codeInternal, Message: err.Error()}
		}
		_ = c.write(message{ID: id, Error: re})
		return
	}
	if result == nil {
		result = struct{}{}
	}
	raw, merr := json.Marshal(result)
	if merr != nil {
		_ = c.write(message{ID: id, Error: &rpcError{Code: codeInternal, Message: merr.Error()}})
		return
	}
	_ = c.write(message{ID: id, Result: raw})
}

// serve reads messages until EOF, dispatching each request on its own goroutine
// so a long prompt never blocks cancellation. It returns after handlers finish.
func (c *conn) serve(in io.Reader, methods map[string]handler, atEOF func()) error {
	r := bufio.NewReaderSize(in, 64<<10)
	for {
		line, tooLong, err := readLine(r)
		if len(bytes.TrimSpace(line)) > 0 || tooLong {
			c.dispatch(line, tooLong, methods)
		}
		if err != nil {
			if atEOF != nil {
				atEOF()
			}
			c.wg.Wait()
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// readLine returns one line without its newline. Longer lines are dropped.
func readLine(r *bufio.Reader) (line []byte, tooLong bool, err error) {
	for {
		part, perr := r.ReadSlice('\n')
		if !tooLong {
			if len(line)+len(part) > MaxLineBytes {
				tooLong, line = true, nil
			} else {
				line = append(line, part...)
			}
		}
		if perr == bufio.ErrBufferFull {
			continue
		}
		return bytes.TrimRight(line, "\r\n"), tooLong, perr
	}
}

func (c *conn) dispatch(line []byte, tooLong bool, methods map[string]handler) {
	if tooLong {
		_ = c.write(message{ID: json.RawMessage("null"), Error: &rpcError{Code: codeInvalidRequest, Message: "message exceeds the size limit"}})
		return
	}
	var m message
	if err := json.Unmarshal(line, &m); err != nil {
		_ = c.write(message{ID: json.RawMessage("null"), Error: &rpcError{Code: codeParse, Message: "parse error"}})
		return
	}
	if m.Method == "" {
		return // a response to a request of ours: none are sent
	}
	h := methods[m.Method]
	isRequest := len(m.ID) > 0
	if h == nil {
		if isRequest {
			c.reply(m.ID, nil, &rpcError{Code: codeNoMethod, Message: "method not found: " + m.Method})
		}
		return
	}
	if isRequest {
		key := string(m.ID)
		c.mu.Lock()
		dup := c.inflight[key]
		c.inflight[key] = true
		c.mu.Unlock()
		if dup {
			c.reply(m.ID, nil, &rpcError{Code: codeInvalidRequest, Message: "duplicate request id " + key})
			return
		}
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer func() {
			if p := recover(); p != nil && isRequest {
				c.reply(m.ID, nil, &rpcError{Code: codeInternal, Message: fmt.Sprint("internal error: ", p)})
			}
		}()
		res, err := h(m.Params)
		if !isRequest {
			return
		}
		c.mu.Lock()
		delete(c.inflight, string(m.ID))
		c.mu.Unlock()
		var then func()
		if r, ok := res.(afterReply); ok {
			res, then = r.value, r.then
		}
		c.reply(m.ID, res, err)
		if then != nil && err == nil {
			then()
		}
	}()
}
