package cli

import "fmt"

func runDaemon(args []string, s IO, a API, j bool) (int, error) {
	if len(args) == 0 {
		return 0, usageError("usage: meno daemon run|start|stop|restart|status|logs")
	}
	if !contains([]string{"run", "start", "stop", "restart", "status", "logs"}, args[0]) {
		return 0, unknown(args[0], []string{"run", "start", "stop", "restart", "status", "logs"})
	}
	values, switches, pos, err := parseFlags(args[1:], nil, map[string]bool{"force": true})
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usageError("unexpected daemon arguments")
	}
	action := args[0]
	if !contains([]string{"run", "start", "stop", "restart", "status", "logs"}, action) {
		return 0, unknown(action, []string{"run", "start", "stop", "restart", "status", "logs"})
	}
	if err = apiRequired(a); err != nil {
		return 0, err
	}
	_ = values
	res, err := a.Daemon(action, switches["force"])
	if err != nil {
		return 0, err
	}
	return 0, emit(s, j, res, daemonText(action, res))
}
func daemonText(action string, d DaemonStatus) string {
	if action == "status" {
		if d.Running {
			return fmt.Sprintf("running (pid %d)\n", d.PID)
		}
		return "stopped\n"
	}
	return action + " requested\n"
}
