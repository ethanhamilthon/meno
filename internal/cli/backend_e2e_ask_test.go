package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"meno/internal/api"
)

func TestAskBlocksUntilAnswered(t *testing.T) {
	_, eng, sess, dapi := start(t, &scripted{})
	form := `{"questions":[{"id":"pick","text":"Which?","type":"single","options":[{"id":"a","label":"A"},{"id":"b","label":"B"}]}]}`
	done := make(chan map[string]any, 1)
	errc := make(chan error, 1)
	go func() {
		out, err := dapi.Ask([]byte(form))
		done <- out
		errc <- err
	}()
	var ask *api.AskForm
	for i := 0; i < 400 && ask == nil; i++ {
		snap, err := eng.Snapshot(context.Background(), sess.Session.ID)
		if err != nil {
			t.Fatal(err)
		}
		ask = snap.Ask
		time.Sleep(5 * time.Millisecond)
	}
	if ask == nil {
		t.Fatal("ask never opened")
	}
	err := eng.AnswerAsk(context.Background(), sess.Session.ID, ask.AskID, api.AskAnswerRequest{Answers: map[string]json.RawMessage{"pick": json.RawMessage(`"b"`)}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case out := <-done:
		if err := <-errc; err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(out)
		if !strings.Contains(string(b), `"b"`) {
			t.Fatalf("answers %s", b)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ask did not return")
	}
}

func TestAskCancelIsAnError(t *testing.T) {
	_, eng, sess, dapi := start(t, &scripted{})
	errc := make(chan error, 1)
	go func() {
		_, err := dapi.Ask([]byte(`{"questions":[{"id":"q","text":"Name?","type":"text"}]}`))
		errc <- err
	}()
	var ask *api.AskForm
	for i := 0; i < 400 && ask == nil; i++ {
		snap, _ := eng.Snapshot(context.Background(), sess.Session.ID)
		ask = snap.Ask
		time.Sleep(5 * time.Millisecond)
	}
	if ask == nil {
		t.Fatal("ask never opened")
	}
	_ = eng.AnswerAsk(context.Background(), sess.Session.ID, ask.AskID, api.AskAnswerRequest{Cancel: true})
	select {
	case err := <-errc:
		var ae *APIError
		if err == nil || !asAPI(err, &ae) || ae.Message != "cancelled" {
			t.Fatalf("err %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ask did not return")
	}
}
