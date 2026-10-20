package daemon

import (
	"fmt"
	"os"
	"path/filepath"
)

const maxLogBytes = 10 << 20

// RotateLog rotates a log at 10 MiB and keeps three prior files.
func RotateLog(path string) error {
	st, e := os.Stat(path)
	if e != nil {
		if os.IsNotExist(e) {
			return nil
		}
		return e
	}
	if st.Size() < maxLogBytes {
		return nil
	}
	_ = os.Remove(path + ".3")
	for i := 2; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", path, i)
		dst := fmt.Sprintf("%s.%d", path, i+1)
		if e := os.Rename(src, dst); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	if e := os.Rename(path, path+".1"); e != nil {
		return e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if e != nil {
		return e
	}
	return f.Close()
}

// RotatePath returns one of the daemon's retained log files by index.
func RotatePath(path string, index int) string {
	if index <= 0 {
		return path
	}
	return filepath.Clean(fmt.Sprintf("%s.%d", path, index))
}
