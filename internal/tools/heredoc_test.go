package tools

import "testing"

func TestFindHeredocBodies(t *testing.T) {
	tests := []struct {
		name, command, body, tag string
		start                    int
	}{
		{"quoted", "cat <<'END'\nbody\nEND\n", "body\n", "END", 12},
		{"body contains redirect text", "cat <<TAG\nbody <<OTHER\nTAG\n", "body <<OTHER\n", "TAG", 10},
		{"plain", "cat <<END\nbody\nEND\n", "body\n", "END", 10},
		{"strip tabs", "cat <<-END\n\tbody\n\tEND\n", "\tbody\n", "END", 11},
		{"after earlier command", "cat; cat <<TAG\nbody\nTAG\n", "body\n", "TAG", 15},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := FindHeredocBodies(test.command)
			if len(got) != 1 {
				t.Fatalf("bodies = %#v", got)
			}
			if got[0].Tag != test.tag || got[0].Body != test.body || got[0].Start != test.start {
				t.Fatalf("body = %#v", got[0])
			}
		})
	}
}

func TestFindHeredocBodiesDoesNotFoldWhenUncertain(t *testing.T) {
	for _, command := range []string{"cat <<TAG", "cat <<<TAG\nbody\nTAG", "echo '<<TAG'\nbody\nTAG", "cat <<TAG\nbody\nOTHER\n"} {
		if got := FindHeredocBodies(command); len(got) != 0 {
			t.Errorf("folded uncertain heredoc %q: %#v", command, got)
		}
	}
}
