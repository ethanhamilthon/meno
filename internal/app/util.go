package app

import (
	"os"
	"strconv"
)

func itoa(n int) string { return strconv.Itoa(n) }

func writeFile(name, s string) error { return os.WriteFile(name, []byte(s), 0o644) }
