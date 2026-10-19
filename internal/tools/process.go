package tools

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 1
}

var processListMu sync.Mutex

func processGroupPIDs(pgid int) []int {
	processListMu.Lock()
	defer processListMu.Unlock()
	cmd := exec.Command("/bin/ps", "-A", "-o", "pid=", "-o", "pgid=", "-o", "stat=")
	data, err := cmd.Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, pErr := strconv.Atoi(fields[0])
		group, gErr := strconv.Atoi(fields[1])
		if pErr == nil && gErr == nil && group == pgid && !strings.HasPrefix(fields[2], "Z") {
			if pid != pgid {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

func killLeftovers(pgid int) int {
	pids := processGroupPIDs(pgid)
	if len(pids) > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	return len(pids)
}
