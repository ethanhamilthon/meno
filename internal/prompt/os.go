package prompt

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// DescribeOS returns e.g. "macOS 15.1 (darwin/arm64)"; when the version
// cannot be found, just "darwin/arm64".
func DescribeOS() string {
	osOnce.Do(func() { osText = describeOS() })
	return osText
}

// The system does not change while meno runs, and finding its version starts a
// process: it is asked once.
var (
	osOnce sync.Once
	osText string
)

func describeOS() string {
	plat := runtime.GOOS + "/" + runtime.GOARCH
	name := ""
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
			if v := strings.TrimSpace(string(out)); v != "" {
				name = "macOS " + v
			}
		}
	case "linux":
		if b, err := os.ReadFile("/etc/os-release"); err == nil {
			for _, l := range strings.Split(string(b), "\n") {
				if v, ok := strings.CutPrefix(l, "PRETTY_NAME="); ok {
					name = strings.Trim(strings.TrimSpace(v), "\"'")
				}
			}
		}
	}
	if name == "" {
		return plat
	}
	return name + " (" + plat + ")"
}
