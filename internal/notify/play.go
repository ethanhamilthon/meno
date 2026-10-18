package notify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Play uses a platform player without a shell. Temporary audio is removed even
// if playback fails or the caller's deadline expires.
func Play(ctx context.Context, name string) error {
	data, err := WAV(name)
	if err != nil {
		return err
	}
	player, args, err := findPlayer(runtime.GOOS, exec.LookPath)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "meno-notify-*.wav")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return exec.CommandContext(ctx, player, append(args, f.Name())...).Run()
}

func findPlayer(goos string, lookup func(string) (string, error)) (string, []string, error) {
	players := []string{"paplay", "aplay", "ffplay"}
	if goos == "darwin" {
		players = []string{"afplay"}
	}
	for _, name := range players {
		if path, err := lookup(name); err == nil {
			var args []string
			if name == "ffplay" {
				args = []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}
			}
			return path, args, nil
		}
	}
	return "", nil, fmt.Errorf("no notification audio player available")
}
