package modes

import "strings"

// Combined is the effect of several modes at once.
type Combined struct {
	Names  []string
	Goal   bool
	Rounds int
	// Only is the intersection of the modes' lists; nil when no mode limits.
	Only []string
	Text string
}

// Compose combines the modes of a message, in the order given. A goal in any
// of them makes the run a goal run. The globals a run keeps are the ones every
// limiting mode keeps. Each mode's text becomes one block of the prompt.
func Compose(list []Mode) Combined {
	var c Combined
	var texts []string
	limited := false
	for _, m := range list {
		c.Names = append(c.Names, m.Name)
		if m.Goal {
			c.Goal = true
			c.Rounds = max(c.Rounds, m.Rounds)
		}
		if m.Only != nil {
			if limited {
				c.Only = intersect(c.Only, m.Only)
			} else {
				c.Only = append([]string{}, m.Only...)
				limited = true
			}
		}
		texts = append(texts, Block(m))
	}
	c.Text = strings.Join(texts, "\n\n")
	return c
}

// intersect keeps the names in both lists.
func intersect(a, b []string) []string {
	in := map[string]bool{}
	for _, n := range b {
		in[n] = true
	}
	out := []string{}
	for _, n := range a {
		if in[n] {
			out = append(out, n)
		}
	}
	return out
}
