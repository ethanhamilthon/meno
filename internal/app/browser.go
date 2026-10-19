package app

import (
	"errors"
	"os/exec"
	"runtime"
)

// openBrowser opens a page in the default browser of the desktop.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return errors.New("cannot open a browser on " + runtime.GOOS)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
