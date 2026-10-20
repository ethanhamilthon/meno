package main

import (
	"context"
	"fmt"
	"time"

	"meno/internal/client"
)

// ensureDaemon connects to the daemon of this MENO_HOME, starting one when
// none runs. The TUI holds no session, provider or process logic of its own.
func ensureDaemon(home string) (*client.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	c, err := client.Ensure(ctx, client.Options{Home: home, Version: version})
	if err != nil {
		return nil, fmt.Errorf("daemon: %w (try: meno daemon restart)", err)
	}
	return c, nil
}
