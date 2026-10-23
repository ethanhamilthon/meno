package config

import (
	"testing"

	"meno/internal/llm"
)

func TestProviderRunAndModelSettingsRoundTripAndFileLoad(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	in := 2.5
	ps := []llm.Provider{{Name: "p", BaseURL: "http://127.0.0.1:1/v1", Protocol: llm.ProtoChat, Run: "proxy --port 1",
		Models: map[string]llm.ModelSettings{"m": {Context: 9000, Input: &in, Efforts: []string{"low", "high"}}}}, {Name: "plain", BaseURL: "http://x", Protocol: llm.ProtoChat}}
	if err := st.SaveProviders(ps); err != nil {
		t.Fatal(err)
	}
	got, err := st.LoadProviders()
	if err != nil || len(got) != 2 || got[0].Run != "proxy --port 1" || got[0].Models["m"].Context != 9000 || *got[0].Models["m"].Input != 2.5 || len(got[0].Models["m"].Efforts) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if got[1].Run != "" || got[1].Models != nil {
		t.Fatalf("a plain provider gained fields: %+v", got[1])
	}
	st2, p2 := storeIn(t)
	writeFile(t, p2.ProviderFile(), `[
		{"name":"f","base_url":"http://127.0.0.1:2/v1","protocol":"responses","run":"serve","models":{"x":{"context":128000,"input":1,"output":2,"efforts":["low","high"]}}},
		{"name":"bad","base_url":"http://y","protocol":"responses","models":{"x":{"context":-5}}}
	]`)
	ps2, warns, _ := st2.LoadProvidersFile()
	if len(ps2) != 1 || ps2[0].Run != "serve" || ps2[0].Models["x"].Context != 128000 || len(warns) != 1 {
		t.Fatalf("%+v %v", ps2, warns)
	}
}
