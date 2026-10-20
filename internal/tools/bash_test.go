package tools

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func runOptions(t *testing.T, command string) RunOptions {
	t.Helper()
	return RunOptions{Command: command, Dir: t.TempDir(), OutputDir: t.TempDir(), CallID: "call"}
}

func TestRunStreamsEnvironmentAndUnifiedOutput(t *testing.T) {
	options := runOptions(t, "printf '%s\n' \"$CUSTOM $TERM $NO_COLOR $PAGER $GIT_PAGER\"; printf err >&2")
	options.Env = []string{"CUSTOM=passed"}
	var mu sync.Mutex
	var streamed strings.Builder
	options.Stream = func(delta string) { mu.Lock(); defer mu.Unlock(); streamed.WriteString(delta) }
	result, err := Run(options)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"passed dumb 1 cat cat", "err", "[exit 0]"} {
		if !strings.Contains(result.Text, want) {
			t.Errorf("result missing %q: %q", want, result.Text)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if streamed.String() != result.Output {
		t.Errorf("stream %q differs from output %q", streamed.String(), result.Output)
	}
	if _, err := os.Stat(result.LogPath); err != nil {
		t.Fatalf("log missing: %v", err)
	}
}

func TestRunExitCodeAndWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	options := RunOptions{Command: "pwd; exit 7", Dir: dir, OutputDir: t.TempDir(), CallID: "exit"}
	result, err := Run(options)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 7 || result.Status != "[exit 7]" {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(result.Output, dir) {
		t.Errorf("cwd missing: %q", result.Output)
	}
}

func TestCancelStopsTheProcessGroup(t *testing.T) {
	call, err := Start(runOptions(t, "trap '' TERM; sleep 10"))
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	call.Cancel()
	result, err := call.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Cancelled || result.Status != "[cancelled]" {
		t.Fatalf("result = %#v", result)
	}
	if time.Since(started) > 7*time.Second {
		t.Fatal("cancel did not kill the process group promptly")
	}
}

func TestRunKillsLeftoverProcess(t *testing.T) {
	options := runOptions(t, "sleep 10 >/dev/null 2>&1 & exit 0")
	result, err := Run(options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Leftovers < 1 || !strings.Contains(result.Status, "[killed ") || !strings.Contains(result.Status, "use meno tasks start for background work]") {
		t.Fatalf("leftover status missing: %#v", result)
	}
}

func TestRunLargeOutputKeepsHeadTailAndLog(t *testing.T) {
	options := runOptions(t, "yes x | head -c 70000")
	result, err := Run(options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Omitted == 0 || !strings.Contains(result.Text, "bytes omitted; full output: "+filepath.Join(options.OutputDir, "call.log")) {
		t.Fatalf("cut marker missing: %#v", result)
	}
	if len(result.Output) > outputHeadLimit+outputTailLimit+512 {
		t.Fatalf("cut result too large: %d", len(result.Output))
	}
	data, err := os.ReadFile(result.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 70000 {
		t.Fatalf("log bytes = %d", len(data))
	}
}

func TestRunLogIsBounded(t *testing.T) {
	options := runOptions(t, "yes z | head -c 12000000")
	result, err := Run(options)
	if err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(result.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() != logLimit {
		t.Fatalf("log size = %d", stat.Size())
	}
}
