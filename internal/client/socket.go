package client

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

func socketPath(home string) string {
	p := filepath.Join(home, "run", "meno.sock")
	if len(p) < 100 {
		return p
	}
	sum := sha256.Sum256([]byte(filepath.Clean(home)))
	return filepath.Join(os.TempDir(), fmt.Sprintf("meno-%d-%x.sock", os.Getuid(), sum[:8]))
}
