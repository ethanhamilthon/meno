package config

import (
	"testing"

	"meno/internal/llm"
)

func TestDisabledProviderRoundTripAndManagedIsNeverSaved(t *testing.T) {
	st := NewStore(Paths{Home: t.TempDir()})
	in := []llm.Provider{
		{Name: "on", BaseURL: "http://a", Protocol: llm.ProtoChat},
		{Name: "off", BaseURL: "http://b", Protocol: llm.ProtoResponses, Disabled: true},
		{Name: llm.OAuthName, BaseURL: "http://c", Source: llm.SourceOAuth},
		{Name: "file", BaseURL: "http://d", Source: llm.SourceFile},
	}
	if err := st.SaveProviders(OwnProviders(in)); err != nil {
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
