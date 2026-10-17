package jsvm

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeAsker struct {
	got   Question
	reply func(ctx context.Context, q Question) (string, error)
}

func (f *fakeAsker) Ask(ctx context.Context, q Question) (string, error) {
	f.got = q
	return f.reply(ctx, q)
}

func TestAskUserReturnsAnswerAndPassesOptions(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	f := &fakeAsker{reply: func(context.Context, Question) (string, error) { return "sqlite", nil }}
	v.SetAsker(f)
	r := run(t, v, "askUser('Which db?', {choices: ['sqlite', 'postgres'], customAnswer: false, timeout: 90})")
	if r.Value != "sqlite" {
		t.Fatalf("%+v", r)
	}
	if f.got.Text != "Which db?" || len(f.got.Choices) != 2 || f.got.CustomAnswer || f.got.Timeout != 90*time.Second {
		t.Fatalf("%+v", f.got)
	}
	run(t, v, "askUser('free text?')")
	if len(f.got.Choices) != 0 || !f.got.CustomAnswer || f.got.Timeout != 0 {
		t.Fatalf("defaults: %+v", f.got)
	}
	run(t, v, "askUser('pick', {choices: ['a']})")
	if !f.got.CustomAnswer {
		t.Fatal("customAnswer defaults to true")
	}
}

func TestAskUserErrors(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	r := v.Exec(context.Background(), "askUser('x')")
	if !strings.Contains(r.Err, "interactive input is not available") {
		t.Fatalf("without an asker: %+v", r)
	}
	v.SetAsker(&fakeAsker{reply: func(context.Context, Question) (string, error) { return "", ErrDeclined }})
	for code, want := range map[string]string{
		"askUser('x')":                               "user declined to answer",
		"askUser()":                                  "question is required",
		"askUser('')":                                "non-empty",
		"askUser('x', {customAnswer: false})":        "requires choices",
		"askUser('x', {chioces: ['a']})":             "unknown option",
		"askUser('x', {choices: 'a'})":               "array of strings",
		"askUser('x', {choices: [1]})":               "non-empty strings",
		"askUser('x', {timeout: -1})":                "negative",
		"askUser('x', {timeout: 'soon'})":            "number of seconds",
		"try { askUser('x') } catch (e) { throw e }": "user declined",
	} {
		if r := v.Exec(context.Background(), code); !strings.Contains(r.Err, want) {
			t.Errorf("%s: got %q, want %q", code, r.Err, want)
		}
	}
	// The model can handle a refusal itself.
	if r := run(t, v, "try { askUser('x'); 'no' } catch (e) { 'caught: ' + e.message }"); !strings.Contains(r.Value, "caught: ") || !strings.Contains(r.Value, "declined") {
		t.Fatalf("%+v", r)
	}
}

func TestAskUserTimeoutIsItsOwnError(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	v.SetAsker(&fakeAsker{reply: func(ctx context.Context, q Question) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}})
	start := time.Now()
	r := v.Exec(context.Background(), "askUser('quick?', {timeout: 0.1})")
	if !strings.Contains(r.Err, "user did not answer within 0.1s") || time.Since(start) > 3*time.Second {
		t.Fatalf("%+v after %v", r, time.Since(start))
	}
}

func TestAskUserWaitsDoNotCountAgainstTheJSTimeout(t *testing.T) {
	v := New(Options{Root: t.TempDir(), Timeout: 300 * time.Millisecond})
	v.SetAsker(&fakeAsker{reply: func(context.Context, Question) (string, error) {
		time.Sleep(700 * time.Millisecond) // longer than the js timeout
		return "late but fine", nil
	}})
	r := v.Exec(context.Background(), "askUser('slow human')")
	if r.Err != "" || r.Value != "late but fine" {
		t.Fatalf("the question's wait burned the js timeout: %+v", r)
	}
	// The remaining time still applies afterwards.
	r = v.Exec(context.Background(), "askUser('again'); while (true) {}")
	if !r.TimedOut {
		t.Fatalf("js timeout must resume after the answer: %+v", r)
	}
}

func TestAskUserCanceledWithTheTurn(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	started := make(chan struct{})
	v.SetAsker(&fakeAsker{reply: func(ctx context.Context, q Question) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-started; cancel() }()
	r := v.Exec(ctx, "askUser('x')")
	if r.Err == "" || strings.Contains(r.Err, "did not answer") {
		t.Fatalf("%+v", r)
	}
}
