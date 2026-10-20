package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func daemonLogs(home string, args []string, out, errOut io.Writer) int {
	p := filepath.Join(home, "log", "daemon.log")
	if len(args) > 0 && args[0] != "-f" {
		fmt.Fprintln(errOut, "meno: daemon logs accepts only -f")
		return 1
	}
	f, e := os.Open(p)
	if e != nil {
		if os.IsNotExist(e) {
			fmt.Fprintln(out, "no daemon log")
			return 0
		}
		fmt.Fprintln(errOut, e)
		return 1
	}
	defer f.Close()
	if len(args) > 0 {
		if _, e = io.Copy(out, f); e != nil {
			fmt.Fprintln(errOut, e)
			return 1
		}
		return 0
	}
	return copyTail(f, out, 200)
}
func copyTail(r io.Reader, w io.Writer, n int) int {
	b, e := io.ReadAll(r)
	if e != nil {
		fmt.Fprintln(w, e)
		return 1
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	_, _ = io.WriteString(w, strings.Join(lines, "\n"))
	return 0
}
