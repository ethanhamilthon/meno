package app

import (
	"context"

	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/proto"
)

// snippets are global and kept by the daemon; these calls are quick, so the UI
// waits for them.
type snippetStore struct{ a *App }

func (a *App) snip() snippetStore { return snippetStore{a} }

func (s snippetStore) SaveSnippet(name, text string) error {
	return s.a.rpc(func(ctx context.Context, c *client.Client) error {
		_, err := c.UpsertSnippet(ctx, name, text)
		return err
	})
}

func (s snippetStore) Snippet(name string) (config.Snippet, error) {
	list, err := s.LoadSnippets()
	if err != nil {
		return config.Snippet{}, err
	}
	for _, x := range list {
		if x.Name == name {
			return x, nil
		}
	}
	return config.Snippet{}, errString("no snippet called " + name)
}

func (s snippetStore) LoadSnippets() ([]config.Snippet, error) {
	var out []config.Snippet
	err := s.a.rpc(func(ctx context.Context, c *client.Client) error {
		r, e := c.Snippets(ctx)
		for _, x := range r.Snippets {
			out = append(out, snippetFromWire(x))
		}
		return e
	})
	return out, err
}

func (s snippetStore) RenameSnippet(old, name string) error {
	return s.a.rpc(func(ctx context.Context, c *client.Client) error {
		_, err := c.RenameSnippet(ctx, old, name)
		return err
	})
}

func (s snippetStore) DeleteSnippet(name string) error {
	return s.a.rpc(func(ctx context.Context, c *client.Client) error { return c.DeleteSnippet(ctx, name) })
}

func snippetFromWire(x proto.ConfigSnippet) config.Snippet {
	return config.Snippet{Name: x.Name, Text: x.Text, UpdatedAt: x.UpdatedAt}
}
