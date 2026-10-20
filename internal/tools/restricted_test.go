package tools

import (
	"strings"
	"testing"
)

func TestParseRestrictedCommand(t *testing.T) {
	allowed := []string{"models", "ask", "goal", "help"}
	quote := string(rune(34))
	tests := []struct {
		name, input string
		accepted    bool
		args        []string
	}{
		{"simple", "meno models", true, []string{"models"}},
		{"single quoted", "meno ask 'one two'", true, []string{"ask", "one two"}},
		{"double quoted", "meno help " + quote + "topic name" + quote, true, []string{"help", "topic name"}},
		{"joined parts", "meno help mo'dels'", true, []string{"help", "models"}},
		{"empty quote", "meno models ''", true, []string{"models", ""}},
		{"newline quoted", "meno ask 'line1\nline2'", true, []string{"ask", "line1\nline2"}},
		{"bare punctuation", "meno goal done summary:=@%+,-", true, []string{"goal", "done", "summary:=@%+,-"}},
		{"semicolon", "meno help; touch pwned", false, nil},
		{"and operator", "meno help && touch pwned", false, nil},
		{"substitution", "meno help $(touch pwned)", false, nil},
		{"backticks", "meno help " + string(rune(96)) + "touch pwned" + string(rune(96)), false, nil},
		{"newline", "meno help\ntouch pwned", false, nil},
		{"redirect", "meno help >pwned", false, nil},
		{"glob", "meno help *", false, nil},
		{"env prefix", "X=1 meno help", false, nil},
		{"path qualified", "./meno help", false, nil},
		{"forbidden group", "meno tasks list", false, nil},
		{"unclosed quote", "meno help 'open", false, nil},
		{"expansion", "meno help " + quote + "$HOME" + quote, false, nil},
		{"tab", "meno\thelp", false, nil},
		{"double quoted backslash", "meno help " + quote + "\\" + quote, false, nil},
		{"single quoted shell syntax is literal", "meno help '$(touch x); echo y'", true, []string{"help", "$(touch x); echo y"}},
		{"bare carriage return", "meno help\r", false, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseRestrictedCommand(test.input, "/usr/local/bin/meno", allowed)
			if (err == nil) != test.accepted {
				t.Fatalf("ParseRestrictedCommand() err = %v", err)
			}
			if err != nil {
				return
			}
			if got.Path != "/usr/local/bin/meno" {
				t.Errorf("path = %q", got.Path)
			}
			if strings.Join(got.Args, "|") != strings.Join(test.args, "|") {
				t.Errorf("args = %#v; want %#v", got.Args, test.args)
			}
		})
	}
}

func TestRestrictedErrorText(t *testing.T) {
	_, err := ParseRestrictedCommand("meno help; echo bad", "/meno", []string{"help"})
	if err == nil || err.Error() != RestrictedCommandError {
		t.Fatalf("error = %v; want %q", err, RestrictedCommandError)
	}
	_, err = ParseRestrictedCommand("meno tasks list", "/meno", []string{"models", "help"})
	if err == nil || err.Error() != "restricted mode: group \"tasks\" is not allowed (allowed: models, help)" {
		t.Fatalf("unexpected group error: %v", err)
	}
}

func FuzzParseRestrictedCommandNeverChangesExecutable(f *testing.F) {
	for _, seed := range []string{"meno help", "meno help; touch /tmp/no", "X=1 meno help", "meno ask 'x y'", "meno help $(id)", "meno help\necho bad"} {
		f.Add(seed)
	}
	const executable = "/daemon/path/meno"
	f.Fuzz(func(t *testing.T, input string) {
		parsed, err := ParseRestrictedCommand(input, executable, []string{"models", "ask", "goal", "help"})
		if err != nil {
			return
		}
		if parsed.Path != executable {
			t.Fatalf("untrusted input changed executable: %q", parsed.Path)
		}
		if len(parsed.Args) == 0 || parsed.Args[0] == "meno" {
			t.Fatalf("unexpected argv: %#v", parsed.Args)
		}
		for _, arg := range parsed.Args {
			if strings.Contains(arg, string(rune(0))) {
				t.Fatalf("NUL reached argv: %#v", parsed.Args)
			}
		}
	})
}

func TestParserRejectsSecondProgram(t *testing.T) {
	second := "second-program-must-not-run"
	backtick := string(rune(96))
	lines := []string{"meno help; " + second, "meno help && " + second, "meno help $(" + second + ")", "meno help " + backtick + second + backtick, "meno help >" + second}
	for _, line := range lines {
		if _, err := ParseRestrictedCommand(line, "/daemon/meno", []string{"help"}); err == nil {
			t.Errorf("accepted injection %q", line)
		}
	}
}
