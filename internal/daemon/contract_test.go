package daemon

import (
	"encoding/json"
	"meno/internal/api"
	"net/http/httptest"
	"testing"
)

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{{&APIError{Code: api.CodeBadRequest}, 400, api.CodeBadRequest}, {&APIError{Code: api.CodeNotFound}, 404, api.CodeNotFound}, {&APIError{Code: api.CodeForbidden}, 403, api.CodeForbidden}, {&APIError{Code: api.CodeConflict}, 409, api.CodeConflict}, {&APIError{Code: api.CodeNotImplemented}, 501, api.CodeNotImplemented}}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		s := &Server{}
		s.result(w, nil, tc.err)
		if w.Code != tc.status {
			t.Errorf("%s status %d", tc.code, w.Code)
		}
		var body api.ErrorEnvelope
		if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil || body.Error.Code != tc.code {
			t.Errorf("%s body %s", tc.code, w.Body.String())
		}
	}
}
