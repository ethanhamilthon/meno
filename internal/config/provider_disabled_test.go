package config

import (
	"testing"

	"meno/internal/llm"
)

func TestDisabledProviderRoundTrip(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	in := []llm.Provider{
		{Name: "on", BaseURL: "http://a", Protocol: llm.ProtoChat},
		{Name: "off", BaseURL: "http://b", Protocol: llm.ProtoResponses, Disabled: true},
	}
	if err := st.SaveProviders(in); err != nil {
		t.Fatal(err)
	}
	got, err := st.LoadProviders()
	if err != nil || len(got) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if got[0].Disabled || !got[1].Disabled || got[1].Name != "off" {
		t.Fatalf("the switch must survive a save: %+v", got)
	}
}
