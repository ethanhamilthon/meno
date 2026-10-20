package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/proto"
)

func TestModelPickerUsesDaemonCredentials(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			http.Error(w, "invalid key", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"shared"},{"id":"only-proxy"}]}`)
	}))
	defer provider.Close()
	home := t.TempDir()
	store := config.NewStore(config.Paths{Home: home})
	if err := store.SaveProviders([]llm.Provider{{Name: "proxy", BaseURL: provider.URL, APIKey: "test-secret", Protocol: llm.ProtoChat,
		Models: map[string]llm.ModelSettings{"shared": {Context: 12345}}}}); err != nil {
		t.Fatal(err)
	}
	rig := startRig(t, home, &recClient{})
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(96, 30)
	a, err := New(scr, config.Paths{Home: home, Cwd: t.TempDir()}, rig.cl)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.st.Providers[0].APIKey != "•" {
		t.Fatal("UI must not receive the credential")
	}
	a.runCommand("model")
	pump(t, a, func() bool { return !a.st.Loading["proxy"] })
	list := a.modelList()
	if len(list) != 2 || list[0].ID != "only-proxy" || list[1].ID != "shared" {
		t.Fatalf("picker models: %+v; errors: %v", list, a.st.ModelErr)
	}
	a.st.Sel = 1
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "shared") || !strings.Contains(text, "12.3k ctx") {
		t.Fatalf("model details missing:\n%s", text)
	}
	a.modelKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	p, m, _ := a.selection()
	if p != "proxy" || m != "shared" {
		t.Fatalf("selection: %s/%s", p, m)
	}
	// A provider added after startup must also appear after its save completes.
	_, err = rig.cl.CreateProvider(context.Background(), proto.ConfigProvider{Name: "later", BaseURL: provider.URL, APIKey: "test-secret", Protocol: llm.ProtoChat})
	if err != nil {
		t.Fatal(err)
	}
	a.onSettingsLoaded(a.fetchConfig())
	pump(t, a, func() bool { return !a.st.Loading["later"] && !a.st.Loading["proxy"] })
	if got := len(a.allModels()); got != 4 {
		t.Fatalf("after provider add: %d models", got)
	}
}

func TestModelsLoadedKeepsStaleListAndProviderMetadata(t *testing.T) {
	a := &App{st: NewState()}
	a.st.Providers = []llm.Provider{{Name: "one", BaseURL: "http://local"}, {Name: "two", BaseURL: "http://local"}}
	contextSize := 1000
	a.onModelsLoaded(modelsLoaded{provider: "one", response: proto.ModelsResponse{Models: []proto.ConfigModel{{Provider: "one", ID: "same", ContextWindow: &contextSize}}}})
	contextSize = 2000
	a.onModelsLoaded(modelsLoaded{provider: "two", response: proto.ModelsResponse{Models: []proto.ConfigModel{{Provider: "two", ID: "same", ContextWindow: &contextSize}}}})
	for i, p := range a.st.Providers {
		info, ok := p.Lookup(a.st.Pricing, "same")
		if !ok || info.Context != (i+1)*1000 {
			t.Fatalf("metadata for %s: %+v", p.Name, info)
		}
	}
	a.onModelsLoaded(modelsLoaded{provider: "one", err: fmt.Errorf("daemon unavailable")})
	if len(a.modelsOf(a.st.Providers[0])) != 1 || a.st.ModelErr["one"] == "" {
		t.Fatal("RPC error must keep the last list")
	}
	a.onModelsLoaded(modelsLoaded{provider: "one", response: proto.ModelsResponse{Models: []proto.ConfigModel{{Provider: "one", ID: "same", Stale: true}}, Errors: map[string]string{"one": "provider offline"}}})
	if len(a.allModels()) != 2 || a.st.ModelErr["one"] != "provider offline" {
		t.Fatal("partial response must keep models and error")
	}
	a.st.Providers[0].Disabled = true
	a.onModelsLoaded(modelsLoaded{provider: "one", response: proto.ModelsResponse{}})
	if len(a.allModels()) != 1 {
		t.Fatal("disabled provider must stay out of the picker")
	}
}

func TestEmptyModelStatus(t *testing.T) {
	a := &App{st: NewState()}
	if got := a.emptyModelText(); !strings.Contains(got, "no active providers") {
		t.Fatal(got)
	}
	a.st.Providers = []llm.Provider{{Name: "proxy"}}
	a.st.Loading["proxy"] = true
	if got := a.emptyModelText(); !strings.Contains(got, "loading") {
		t.Fatal(got)
	}
	delete(a.st.Loading, "proxy")
	a.st.ModelErr["proxy"] = "invalid key"
	if got := a.emptyModelText(); got != "proxy: invalid key" {
		t.Fatal(got)
	}
}
