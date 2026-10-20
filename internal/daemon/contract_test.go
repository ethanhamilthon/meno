package daemon

import (
	"encoding/json"
	"meno/internal/proto"
	"net/http/httptest"
	"testing"
)

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{{&APIError{Code: proto.CodeBadRequest}, 400, proto.CodeBadRequest}, {&APIError{Code: proto.CodeNotFound}, 404, proto.CodeNotFound}, {&APIError{Code: proto.CodeForbidden}, 403, proto.CodeForbidden}, {&APIError{Code: proto.CodeConflict}, 409, proto.CodeConflict}, {&APIError{Code: proto.CodeProtocolMismatch}, 426, proto.CodeProtocolMismatch}, {&APIError{Code: proto.CodeDaemonStopping}, 503, proto.CodeDaemonStopping}, {&APIError{Code: proto.CodeNotImplemented}, 501, proto.CodeNotImplemented}}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		s := &Server{}
		s.result(w, nil, tc.err)
		if w.Code != tc.status {
			t.Errorf("%s status %d", tc.code, w.Code)
		}
		var body proto.ErrorEnvelope
		if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil || body.Error.Code != tc.code {
			t.Errorf("%s body %s", tc.code, w.Body.String())
		}
	}
}
