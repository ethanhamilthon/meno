// Package jsonrpc is newline-delimited JSON-RPC 2.0 over a byte stream: framing,
// dispatch and notifications. It knows no method of any protocol.
package jsonrpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// MaxLineBytes bounds one incoming JSON-RPC message.
const MaxLineBytes = 16 << 20

// JSON-RPC error codes.
const (
	CodeParse          = -32700
	CodeInvalidRequest = -32600
	CodeNoMethod       = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
)

// Error is a JSON-RPC error object; handlers return it to choose the code.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string { return e.Message }

func ErrParams(format string, a ...any) error {
	return &Error{Code: CodeInvalidParams, Message: fmt.Sprintf(format, a...)}
}

type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Handler serves one request or notification. A returned value becomes the result.
type Handler func(params json.RawMessage) (any, error)

// AfterReply is a handler result that runs then once the reply is written, so
// a notification can follow its response in order on the wire.
type AfterReply struct {
	Value any
	Then  func()
}

// Conn is a newline-delimited JSON-RPC 2.0 connection. Writes are serialized.
type Conn struct {
	// Notifications lists the methods that run when sent as notifications (no
	// id). Any other method sent without an id is ignored.
	Notifications map[string]bool
	// OnHandled, if set, is told how every handled call ended.
	OnHandled func(method string, code int, start time.Time)
	out       io.Writer
	wmu       sync.Mutex

	mu       sync.Mutex
	inflight map[string]bool
	wg       sync.WaitGroup
}

func NewConn(out io.Writer) *Conn {
	return &Conn{out: out, inflight: map[string]bool{}}
}

func (c *Conn) write(m Message) error {
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

// Wait blocks until every running handler has returned.
func (c *Conn) Wait() { c.wg.Wait() }

// Notify sends a notification to the client.
func (c *Conn) Notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(Message{Method: method, Params: raw})
}

func (c *Conn) reply(id json.RawMessage, result any, err error) {
	if err != nil {
		var re *Error
		if !errors.As(err, &re) {
			re = &Error{Code: CodeInternal, Message: "operation failed"}
		}
		_ = c.write(Message{ID: id, Error: re})
		return
	}
	if result == nil {
		result = struct{}{}
	}
	raw, merr := json.Marshal(result)
	if merr != nil {
		_ = c.write(Message{ID: id, Error: &Error{Code: CodeInternal, Message: "response encoding failed"}})
		return
	}
	_ = c.write(Message{ID: id, Result: raw})
}

// serve reads messages until EOF, dispatching each request on its own goroutine
// so a long prompt never blocks cancellation. It returns after handlers finish.
func (c *Conn) Serve(in io.Reader, methods map[string]Handler, atEOF func()) error {
	r := bufio.NewReaderSize(in, 64<<10)
	for {
		line, tooLong, err := readLine(r)
		if len(bytes.TrimSpace(line)) > 0 || tooLong {
			c.Dispatch(line, tooLong, methods)
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

func (c *Conn) Dispatch(line []byte, tooLong bool, methods map[string]Handler) {
	if tooLong {
		_ = c.write(Message{ID: json.RawMessage("null"), Error: &Error{Code: CodeInvalidRequest, Message: "message exceeds the size limit"}})
		return
	}
	if !json.Valid(line) {
		_ = c.write(Message{ID: json.RawMessage("null"), Error: &Error{Code: CodeParse, Message: "parse error"}})
		return
	}
	var m Message
	if json.Unmarshal(line, &m) != nil || m.JSONRPC != "2.0" || !validID(m.ID) || m.Method == "" {
		_ = c.write(Message{ID: json.RawMessage("null"), Error: &Error{Code: CodeInvalidRequest, Message: "invalid request"}})
		return
	}
	// Request-only methods must not cause side effects when sent as notifications.
	if len(m.ID) == 0 && !c.Notifications[m.Method] {
		return
	}
	h := methods[m.Method]
	isRequest := len(m.ID) > 0
	if h == nil {
		if isRequest {
			c.reply(m.ID, nil, &Error{Code: CodeNoMethod, Message: "method not found"})
		}
		return
	}
	if isRequest {
		key := string(m.ID)
		c.mu.Lock()
		dup := c.inflight[key]
		if !dup {
			c.inflight[key] = true
		}
		c.mu.Unlock()
		if dup {
			c.reply(m.ID, nil, &Error{Code: CodeInvalidRequest, Message: "duplicate request id"})
			return
		}
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if isRequest {
			defer func() { c.mu.Lock(); delete(c.inflight, string(m.ID)); c.mu.Unlock() }()
		}
		defer func() {
			if p := recover(); p != nil && isRequest {
				c.reply(m.ID, nil, &Error{Code: CodeInternal, Message: "internal error"})
			}
		}()
		start := time.Now()
		res, err := h(m.Params)
		code := 0
		if err != nil {
			code = CodeInternal
			var re *Error
			if errors.As(err, &re) {
				code = re.Code
			}
		}
		if c.OnHandled != nil {
			c.OnHandled(m.Method, code, start)
		}
		if !isRequest {
			return
		}
		var then func()
		if r, ok := res.(AfterReply); ok {
			res, then = r.Value, r.Then
		}
		c.reply(m.ID, res, err)
		if then != nil && err == nil {
			then()
		}
	}()
}

// validID rejects compound and fractional identifiers before dispatch.
func validID(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	if string(raw) == "null" {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return true
	}
	var n json.Number
	if json.Unmarshal(raw, &n) != nil {
		return false
	}
	_, err := n.Int64()
	return err == nil
}
