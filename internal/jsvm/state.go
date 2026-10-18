package jsvm

import (
	_ "embed"
	"errors"

	"github.com/dop251/goja"
)

// StateEntry is one key of the persistent state with its encoded JSON value.
type StateEntry struct{ Key, JSON string }

// StateStore is the durable per-session JSON map behind the state global. Set
// is atomic and durable when it returns; Entries is a consistent snapshot in
// byte-wise key order. Errors surface in JS as exceptions.
type StateStore interface {
	Set(key, json string) error
	Get(key string) (string, bool, error)
	Delete(key string) (bool, error)
	Entries() ([]StateEntry, error)
}

//go:embed state.js
var stateJS string

//go:embed ask.js
var askJS string

var (
	preludeProg = goja.MustCompile("prelude.js", preludeJS, false)
	stateProg   = goja.MustCompile("state.js", stateJS, false)
	askProg     = goja.MustCompile("ask.js", askJS, false)
)

// SetState enables the state global for this VM.
func (v *VM) SetState(s StateStore) { v.state.Store(&s) }

func (v *VM) store() (StateStore, error) {
	if p := v.state.Load(); p != nil {
		return *p, nil
	}
	return nil, errors.New("state is not available")
}

// bindState installs the raw __state functions that state.js wraps. Values
// cross the boundary as validated JSON text, never as live objects.
func (v *VM) bindState() {
	rt := v.rt
	obj := rt.NewObject()
	_ = obj.Set("set", func(key, json string) error {
		s, err := v.store()
		if err != nil {
			return err
		}
		return s.Set(key, json)
	})
	_ = obj.Set("get", func(key string) (any, error) {
		s, err := v.store()
		if err != nil {
			return nil, err
		}
		json, ok, err := s.Get(key)
		if err != nil || !ok {
			return nil, err
		}
		return json, nil
	})
	_ = obj.Set("delete", func(key string) (bool, error) {
		s, err := v.store()
		if err != nil {
			return false, err
		}
		return s.Delete(key)
	})
	_ = obj.Set("entries", func() ([][2]string, error) {
		s, err := v.store()
		if err != nil {
			return nil, err
		}
		es, err := s.Entries()
		out := make([][2]string, len(es))
		for i, e := range es {
			out[i] = [2]string{e.Key, e.JSON}
		}
		return out, err
	})
	_ = rt.Set("__state", obj)
}
