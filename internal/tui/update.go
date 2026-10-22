package tui

import (
	"context"
	"time"

	"meno/internal/safe"
)

// updateAvailable carries the tag of a newer release from the check to the UI.
type updateAvailable struct{ tag string }

// SetUpdateCheck sets the function Run calls once in the background to learn
// about a newer release: it returns the tag, or "" when there is none.
func (a *App) SetUpdateCheck(f func(ctx context.Context) string) { a.checkUpdate = f }

func (a *App) startUpdateCheck() {
	if a.checkUpdate == nil {
		return
	}
	safe.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		go func() {
			select {
			case <-a.done:
				cancel()
			case <-ctx.Done():
			}
		}()
		if tag := a.checkUpdate(ctx); tag != "" {
			a.postEvent(updateAvailable{tag})
		}
	}, a.reportPanic)
}

func (a *App) onUpdateAvailable(tag string) {
	a.st.UpdateTag = tag
	a.notify("meno " + tag + " is available: run 'meno update' in a terminal")
}
