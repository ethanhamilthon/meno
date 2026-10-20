package modes

import "strings"

// Combined is the effect of several modes at once.
type Combined struct {
	Names  []string
	Goal   bool
	Rounds int
	// Only is the intersection of the modes' lists; nil when no mode limits.
	Only []string
	// Allow is the intersection of command groups; nil allows all groups.
	Allow []string
	Text  string
}

// Compose combines modes in order. Goal is enabled when any mode requests it;
// globals and groups are intersections of limiting modes. Text becomes prompt blocks.
func Compose(list []Mode) Combined {
	var c Combined
	var texts []string
	limited := false
	groupsLimited := false
	for _, m := range list {
		c.Names = append(c.Names, m.Name)
		if m.Goal {
			c.Goal = true
			c.Rounds = max(c.Rounds, m.Rounds)
		}
		if m.Allow != nil {
			if !groupsLimited {
				c.Allow = append([]string{}, m.Allow...)
				groupsLimited = true
			} else {
				c.Allow = intersect(c.Allow, m.Allow)
			}
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

// Allows reports whether a command group is enabled by this mode set.
func (c Combined) Allows(group string) bool {
	return Allows(c.Allow, group)
}

// Allows reports whether a group is in a policy; nil allows every group.
func Allows(groups []string, group string) bool {
	return contains(groups, group)
}

// AllowedGroups returns a copy of the restricted groups, or nil when all groups are enabled.
func (c Combined) AllowedGroups() []string {
	if c.Allow == nil {
		return nil
	}
	return append([]string{}, c.Allow...)
}

func contains(list []string, value string) bool {
	if list == nil {
		return true
	}
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
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
