package app

import (
	"testing"

	"meno/internal/llm"
)

func TestProviderRunStartsAtLaunchAndStopsAtClose(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	a.st.Providers = append(a.st.Providers, llm.Provider{Name: "proxy", BaseURL: "http://127.0.0.1:1", Protocol: llm.ProtoChat, Run: "sleep 60"})
	if a.daemons.Running("proxy") {
		t.Fatal("nothing may start before Run")
	}
	a.startProviderRuns()
	if !a.daemons.Running("proxy") {
		t.Fatal("the run command did not start")
	}
	if a.daemons.Running("openai") {
		t.Fatal("a provider without run must not start anything")
	}
	a.Close()
	if a.daemons.Running("proxy") {
		t.Fatal("the proxy must stop with meno")
	}
}

func TestSavingAProviderSyncsItsRunCommand(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	defer a.daemons.StopAll()
	a.st.EditIdx = -1
	a.st.EditProv = llm.Provider{Name: "p2", BaseURL: "http://127.0.0.1:2", Protocol: llm.ProtoChat, Run: "sleep 60"}
	a.saveProviderEdit()
	if !a.daemons.Running("p2") {
		t.Fatal("saving did not start the run command")
	}
	a.st.EditIdx = len(a.st.Providers) - 1
	a.st.EditProv.Run = ""
	a.saveProviderEdit()
	if a.daemons.Running("p2") {
		t.Fatal("an emptied run command must stop the daemon")
	}
}

func TestProviderWithRunGetsMoreRetries(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	if got := a.providerRetry(llm.Provider{}).Attempts; got != 0 {
		t.Fatalf("the default stays the default, got %d", got)
	}
	if got := a.providerRetry(llm.Provider{Run: "x"}).Attempts; got < 8 {
		t.Fatalf("a starting proxy needs more attempts, got %d", got)
	}
	a.retry.Attempts = 2
	if got := a.providerRetry(llm.Provider{Run: "x"}).Attempts; got != 2 {
		t.Fatalf("an explicit policy wins, got %d", got)
	}
}

func TestModelEffortsFromTheProviderLimitCyclingAndRequests(t *testing.T) {
	cl := &recClient{}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.st.Providers[0].Models = map[string]llm.ModelSettings{"gpt-5.5": {Efforts: []string{"fast", "slow"}}}
	if got := a.effortsFor("openai", "gpt-5.5"); len(got) != 2 || got[0] != "fast" {
		t.Fatal(got)
	}
	if got := a.effortsFor("openai", "other"); len(got) != 4 {
		t.Fatalf("a model without settings keeps the standard levels: %v", got)
	}
	say(t, a, "hello") // the session has effort "high": the model does not take it
	if got := cl.requests()[0].Effort; got != "fast" {
		t.Fatalf("effort sent: %q", got)
	}
	a.setSelection("openai", "gpt-5.5", "fast")
	a.cycleEffort(1, nil)
	if _, _, e := a.selection(); e != "slow" {
		t.Fatalf("cycled to %q", e)
	}
	a.cycleEffort(1, nil)
	if _, _, e := a.selection(); e != "fast" {
		t.Fatalf("cycling must wrap inside the model's levels, got %q", e)
	}
}
