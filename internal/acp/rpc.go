// Package acp serves the Agent Client Protocol (ACP) over a byte stream. It
// translates ACP requests into calls on the engine client and engine events
// into ACP session updates. It knows no client product and holds no agent loop,
// storage or terminal code.
package acp

import (
	"io"

	"meno/internal/jsonrpc"
)

// MaxLineBytes bounds one incoming JSON-RPC message.
const MaxLineBytes = jsonrpc.MaxLineBytes

const (
	codeParse          = jsonrpc.CodeParse
	codeInvalidRequest = jsonrpc.CodeInvalidRequest
	codeNoMethod       = jsonrpc.CodeNoMethod
	codeInvalidParams  = jsonrpc.CodeInvalidParams
	codeInternal       = jsonrpc.CodeInternal
)

type (
	rpcError   = jsonrpc.Error
	message    = jsonrpc.Message
	handler    = jsonrpc.Handler
	afterReply = jsonrpc.AfterReply
	conn       = jsonrpc.Conn
)

var errParams = jsonrpc.ErrParams

// newConn creates a connection on which session/cancel is also valid as a notification.
func newConn(out io.Writer) *conn {
	c := jsonrpc.NewConn(out)
	c.Notifications = map[string]bool{"session/cancel": true}
	return c
}
