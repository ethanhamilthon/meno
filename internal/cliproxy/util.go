package cliproxy

import (
	"os"
	"strconv"
	"strings"
)

func itoa(n int) string { return strconv.Itoa(n) }

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// shellQuote makes s one word for /bin/sh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// yamlQuote makes s a YAML double-quoted string.
func yamlQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + r.Replace(s) + `"`
}
