package client

import "strings"

func daemonEnv(env []string, home string) []string {
	out := make([]string, 0, len(env)+1)
	for _, value := range env {
		if !strings.HasPrefix(value, "MENO_HOME=") {
			out = append(out, value)
		}
	}
	return append(out, "MENO_HOME="+home)
}
