package client

import (
	"fmt"
	"os"
)

func rotateClientLog(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Size() < 10<<20 {
		return nil
	}
	_ = os.Remove(path + ".3")
	for i := 2; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", path, i)
		dst := fmt.Sprintf("%s.%d", path, i+1)
		if err = os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(path, path+".1")
}
