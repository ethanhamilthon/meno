package jsvm

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeAsker struct {
	form    Form
	answers []FormAnswer
	err     error
	wait    bool
}

func (f *fakeAsker) Ask(ctx context.Context, form Form) ([]FormAnswer, error) {
	f.form = form
	if f.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.answers, f.err
}

const oneForm = `{questions: [
	{id: "db", text: "Which database?", type: "single", options: [{id: "pg", label: "Postgres"}, {id: "sq", label: "SQLite"}], recommendation: "Postgres"},
	{id: "feat", text: "Features?", type: "multi", options: [{id: "a", label: "A"}, {id: "b", label: "B"}]},
	{id: "name", text: "Name?", type: "text"},
], timeout: 5}`

func TestAskUserFormRoundTrip(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	fa := &fakeAsker{answers: []FormAnswer{{ID: "db", Selected: []string{"pg"}}, {ID: "feat", Selected: []string{"a", "b"}, Text: "C too"}, {ID: "name", Text: "meno"}}}
	v.SetAsker(fa)
	r := ex(t, v, "const r = askUser("+oneForm+"); JSON.stringify(r)")
	if r.Err != "" {
		t.Fatal(r.Err)
	}
	if !strings.Contains(r.Value, `"id":"db"`) && !strings.Contains(r.Value, `"id": "db"`) {
		t.Fatalf("%s", r.Value)
	}
	if len(fa.form.Questions) != 3 || fa.form.Timeout != 5*time.Second || fa.form.Questions[0].Recommendation != "Postgres" || fa.form.Questions[1].Options[1].Label != "B" {
		t.Fatalf("form: %+v", fa.form)
	}
	r = ex(t, v, "const r = askUser("+oneForm+"); r.answers[1].selected.join('+') + '|' + r.answers[1].text + '|' + r.answers[2].text")
	if r.Value != "a+b|C too|meno" {
		t.Fatalf("%+v", r)
	}
}

func TestAskUserValidatesBeforeShowingAnything(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	fa := &fakeAsker{}
	v.SetAsker(fa)
	opts := `options: [{id: "a", label: "A"}]`
	cases := map[string]string{
		"askUser('plain string')":  "pass {questions",
		"askUser({})":              "at least one question",
		"askUser({questions: []})": "at least one question",
		"askUser({questions: [{id: 'a', text: 't', type: 'text'}, {id: 'a', text: 't', type: 'text'}]})":                                         "duplicate question id",
		"askUser({questions: [{id: '', text: 't', type: 'text'}]})":                                                                              "non-empty string id",
		"askUser({questions: [{id: 'a', text: 't', type: 'radio'}]})":                                                                            "single, multi or text",
		"askUser({questions: [{id: 'a', text: 't', type: 'single'}]})":                                                                           "needs options",
		"askUser({questions: [{id: 'a', text: 't', type: 'text', " + opts + "}]})":                                                               "takes no options",
		"askUser({questions: [{id: 'a', text: 't', type: 'single', options: [{id: 'x', label: 'X'}, {id: 'x', label: 'Y'}]}]})":                  "two options with id",
		"askUser({questions: [{id: 'a', text: 't', type: 'single', options: [{id: 'x'}]}]})":                                                     "non-empty label",
		"askUser({questions: [{id: 'a', text: 't', type: 'text', extra: 1}]})":                                                                   "unknown field",
		"askUser({questions: [{id: 'a', text: 't', type: 'text'}], timeout: -1})":                                                                "finite number",
		"askUser({questions: [{id: 'a', text: 't', type: 'text'}], timeout: NaN})":                                                               "finite number",
		"askUser({questions: [{id: 'a', text: 't', type: 'text'}], mode: 1})":                                                                    "unknown option",
		"askUser({questions: Array.from({length: 21}, (_, i) => ({id: 'q' + i, text: 't', type: 'text'}))})":                                     "at most 20",
		"askUser({questions: [{id: 'a', text: 't', type: 'single', options: Array.from({length: 21}, (_, i) => ({id: 'o' + i, label: 'L'}))}]})": "more than 20 options",
	}
	for code, frag := range cases {
		r := ex(t, v, code)
		if !strings.Contains(r.Err, frag) {
			t.Errorf("%s => %+v, want %q", code, r, frag)
		}
	}
	if len(fa.form.Questions) != 0 {
		t.Fatal("an invalid form must not reach the UI")
	}
}

func TestAskUserCancelAndTimeoutAreCatchableTypedErrors(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	one := `{questions: [{id: "a", text: "t", type: "text"}]}`
	v.SetAsker(&fakeAsker{err: ErrCancelled})
	r := ex(t, v, "try { askUser("+one+"); 'answered' } catch (e) { (e instanceof AskCancelled) + ':' + e.name + ':' + (e instanceof Error) }")
	if r.Value != "true:AskCancelled:true" {
		t.Fatalf("%+v", r)
	}
	// Uncaught, it ends the call with an error naming the cancellation.
	if r := ex(t, v, "askUser("+one+"); console.log('not reached')"); !strings.Contains(r.Err, "cancelled") || strings.Contains(r.Output, "not reached") {
		t.Fatalf("%+v", r)
	}
	v.SetAsker(&fakeAsker{err: ErrTimeout})
	r = ex(t, v, "try { askUser("+one+") } catch (e) { e.name }")
	if r.Value != "AskTimeout" {
		t.Fatalf("%+v", r)
	}
	// The real deadline: the asker waits for ctx and the timeout turns into AskTimeout.
	v.SetAsker(&fakeAsker{wait: true})
	r = ex(t, v, "try { askUser({questions: [{id: 'a', text: 't', type: 'text'}], timeout: 0.05}) } catch (e) { e.name }")
	if r.Value != "AskTimeout" {
		t.Fatalf("deadline: %+v", r)
	}
}

func TestAskUserWaitDoesNotCountAgainstJSTimeout(t *testing.T) {
	v := New(Options{Root: t.TempDir(), Timeout: 150 * time.Millisecond})
	v.SetAsker(&slowAsker{d: 400 * time.Millisecond})
	r := ex(t, v, "askUser({questions: [{id: 'a', text: 't', type: 'text'}]}).answers[0].text")
	if r.Err != "" || r.Value != "late" {
		t.Fatalf("human time must not hit the js timeout: %+v", r)
	}
}

type slowAsker struct{ d time.Duration }

func (s *slowAsker) Ask(ctx context.Context, f Form) ([]FormAnswer, error) {
	select {
	case <-time.After(s.d):
		return []FormAnswer{{ID: "a", Text: "late"}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestAskUserParentCancellationPropagates(t *testing.T) {
	v := New(Options{Root: t.TempDir()})
	v.SetAsker(&fakeAsker{wait: true})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Result, 1)
	go func() {
		done <- v.Exec(ctx, "try { askUser({questions: [{id: 'a', text: 't', type: 'text'}]}); } catch (e) { console.log('caught ' + e.name) }")
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	r := <-done
	if strings.Contains(r.Output, "AskCancelled") || strings.Contains(r.Output, "AskTimeout") {
		t.Fatalf("parent cancellation must not look like a user cancel: %+v", r)
	}
}
