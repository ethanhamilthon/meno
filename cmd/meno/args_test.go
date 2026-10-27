package main

import "testing"

func TestParseLaunch(t *testing.T) {
	cases := []struct {
		args    []string
		ok      bool
		want    launch
		wantErr bool
	}{
		{nil, true, launch{}, false},
		{[]string{"--no-ask"}, true, launch{noAsk: true}, false},
		{[]string{"mcp", "list"}, false, launch{}, false},
		{[]string{"--help"}, false, launch{}, false},
		{[]string{"resume"}, true, launch{picker: true}, false},
		{[]string{"resume", "abc"}, true, launch{resume: "abc"}, false},
		{[]string{"--continue"}, true, launch{cont: true}, false},
		{[]string{"-p", "fix", "the", "bug"}, true, launch{print: true, prompt: "fix the bug", hasInput: true}, false},
		{[]string{"-p", "--model", "p/m", "--resume", "s1", "go"}, true, launch{print: true, model: "p/m", resume: "s1", prompt: "go", hasInput: true}, false},
		{[]string{"-p", "--", "--literal"}, true, launch{print: true, prompt: "--literal", hasInput: true}, false},
		{[]string{"-p", "--stdio"}, true, launch{print: true, stdio: true}, false},
		{[]string{"--stdio"}, true, launch{}, true},
		{[]string{"-p", "--stdio", "hello"}, true, launch{}, true},
		{[]string{"--resume", "a", "--continue"}, true, launch{}, true},
		{[]string{"--resume"}, true, launch{}, true},
		{[]string{"-p", "--no-save", "--resume", "x", "go"}, true, launch{}, true},
		{[]string{"-p", "--no-save", "--continue", "go"}, true, launch{}, true},
		{[]string{"--json"}, true, launch{}, true},
		{[]string{"-p", "--bogus"}, true, launch{}, true},
	}
	for _, c := range cases {
		got, ok, err := parseLaunch(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("%v: error %v, want error %v", c.args, err, c.wantErr)
			continue
		}
		if c.wantErr {
			continue
		}
		if ok != c.ok || got != c.want {
			t.Errorf("%v: got %+v ok=%v, want %+v ok=%v", c.args, got, ok, c.want, c.ok)
		}
	}
}
