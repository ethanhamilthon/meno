package jsvm

import (
	"strings"
	"testing"
	"time"
)

type fakeTasks struct {
	call    TaskCall
	command string
}

func (f *fakeTasks) Start(c TaskCall, command string) (any, error) {
	f.call, f.command = c, command
	return map[string]any{"id": "t1"}, nil
}
func (fakeTasks) List(TaskCall) (any, error)                        { return []any{}, nil }
func (fakeTasks) Status(TaskCall, string) (any, error)              { return nil, nil }
func (fakeTasks) Logs(TaskCall, string) (any, error)                { return nil, nil }
func (fakeTasks) Wait(TaskCall, string, time.Duration) (any, error) { return nil, nil }
func (fakeTasks) Cancel(TaskCall, string) (any, error)              { return nil, nil }

func TestTaskEnvOverridesReplaceValues(t *testing.T) {
	t.Setenv("MENO_KEEP", "old")
	t.Setenv("MENO_OVERRIDE", "plain")
	v := New(Options{Root: t.TempDir()})
	f := &fakeTasks{}
	v.SetTasks(f)
	run(t, v, "tasks.start('echo hi', {env: {MENO_OVERRIDE: 'new', MENO_ADD: 'added'}})")
	if f.command != "echo hi" {
		t.Fatalf("command %q", f.command)
	}
	got := map[string]string{}
	for _, kv := range f.call.Env {
		k, val, ok := strings.Cut(kv, "=")
		if !ok {
			t.Fatalf("bad env entry %q", kv)
		}
		if _, twice := got[k]; twice {
			t.Fatalf("duplicate env %q in %v", k, f.call.Env)
		}
		got[k] = val
	}
	if got["MENO_KEEP"] != "old" || got["MENO_OVERRIDE"] != "new" || got["MENO_ADD"] != "added" {
		t.Fatalf("env %+v", got)
	}
}
