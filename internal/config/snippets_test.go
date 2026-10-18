package config

import "testing"

func snippetStore(t *testing.T) *Store {
	t.Helper()
	home := t.TempDir()
	return NewStore(Paths{Home: home, Cwd: home})
}

func TestSnippetsSaveLoadRenameDelete(t *testing.T) {
	s := snippetStore(t)
	if list, err := s.LoadSnippets(); err != nil || len(list) != 0 {
		t.Fatalf("initial list %v %v", list, err)
	}
	if err := s.SaveSnippet("greet", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSnippet("later", "second"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSnippet("greet", "hello again"); err != nil {
		t.Fatal(err)
	}
	list, err := s.LoadSnippets()
	if err != nil || len(list) != 2 {
		t.Fatalf("list %v %v", list, err)
	}
	if list[0].Name != "greet" || list[0].Text != "hello again" {
		t.Fatalf("sorted/updated snippet %+v", list[0])
	}
	if err := s.RenameSnippet("greet", "hi"); err != nil {
		t.Fatal(err)
	}
	sn, err := s.Snippet("hi")
	if err != nil || sn.Text != "hello again" {
		t.Fatalf("renamed %+v %v", sn, err)
	}
	if _, err := s.Snippet("greet"); err == nil {
		t.Fatal("old name still resolves")
	}
	if err := s.RenameSnippet("hi", "later"); err == nil {
		t.Fatal("rename onto a taken name must fail")
	}
	if err := s.RenameSnippet("missing", "x"); err == nil {
		t.Fatal("rename of a missing snippet must fail")
	}
	if err := s.DeleteSnippet("later"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSnippet("later"); err != nil {
		t.Fatalf("deleting twice must be quiet: %v", err)
	}
	if list, err := s.LoadSnippets(); err != nil || len(list) != 1 || list[0].Name != "hi" {
		t.Fatalf("after delete %v %v", list, err)
	}
}

func TestSnippetNames(t *testing.T) {
	for _, name := range []string{"a", "greet-2", "upper_low", "v1.2"} {
		if err := CheckSnippetName(name); err != nil {
			t.Errorf("%q rejected: %v", name, err)
		}
	}
	for _, name := range []string{"", "two words", "slash/name", "**"} {
		if err := CheckSnippetName(name); err == nil {
			t.Errorf("%q accepted", name)
		}
	}
	s := snippetStore(t)
	if err := s.SaveSnippet("bad name", "x"); err == nil {
		t.Fatal("store accepted an invalid name")
	}
}
